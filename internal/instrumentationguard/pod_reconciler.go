// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// PodReconciler watches auto-instrumented pods and, when one of them breaks shortly after it
// starts AND the crash output holds auto-instrumentation responsible, emits a Warning Event on the
// workload that owns it. It is read-only on customer objects: the Event is the whole output, and
// the workload is read only to address the Event to it and to see whether the customer has opted
// out. See the package comment for why the guard stops there.
type PodReconciler struct {
	client   client.Client
	logger   logr.Logger
	recorder record.EventRecorder
	cfg      Config
	now      func() time.Time
}

// NewPodReconciler creates a PodReconciler. See PodReconciler.
func NewPodReconciler(c client.Client, logger logr.Logger, recorder record.EventRecorder, cfg Config) *PodReconciler {
	return &PodReconciler{
		client:   c,
		logger:   logger,
		recorder: recorder,
		cfg:      cfg,
		now:      time.Now,
	}
}

// Reconcile evaluates one auto-instrumented pod and, if it is broken and the failure is
// attributable to auto-instrumentation, reports it on the workload that owns it. EvaluatePod
// decides whether the pod is broken; AttributeFailure decides whether that is our doing. Both must
// say yes before anything is emitted, and nothing is ever written.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := corev1.Pod{}
	if err := r.client.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	images := instrumentation.InjectedImages(pod)
	if len(images) == 0 {
		// Not auto-instrumented by this operator; nothing to attribute a failure to.
		return ctrl.Result{}, nil
	}

	verdict := EvaluatePod(pod, r.now(), r.cfg)
	if !verdict.Broken {
		if InWindow(pod, r.now(), r.cfg.Window) {
			// A crash-looping pod keeps producing status updates, but a pod stuck in init may not,
			// so re-check it until it leaves the window.
			return ctrl.Result{RequeueAfter: r.cfg.RequeueInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	// The pod is broken, but plenty of pods break for reasons of their own. Attribution runs
	// before the owner walk, which issues retrying API Gets, so an unattributed failure costs
	// nothing beyond reading the pod we already have.
	attribution := AttributeFailure(pod, configuredImages(images))
	if !attribution.Ours {
		r.logger.V(1).Info("instrumentation guard will not act on a broken pod it cannot attribute to auto-instrumentation",
			"pod", podName(pod), "verdict", verdict.Reason, "attribution", attribution.Reason)
		// No requeue: the evidence does not improve by looking again. A pod that produces new
		// evidence produces a new status update, and that is what wakes us.
		return ctrl.Result{}, nil
	}
	reason := fmt.Sprintf("%s; %s", verdict.Reason, attribution.Reason)

	workload, err := ResolveWorkload(ctx, r.client, pod)
	if err != nil {
		return ctrl.Result{}, err
	}
	if workload == nil {
		return ctrl.Result{}, nil
	}
	if HasRecord(workload) && injectionDisabledOnTemplate(workload, images) {
		// The guard writes nothing, so the record annotation is only ever the customer's: it
		// marks a workload they have already dealt with. It silences the guard only while the
		// pod template still has injection turned off for every language in this pod - the state
		// the customer silenced it about. A workload whose inject annotations have come back to
		// "true", which is what kubectl apply and GitOps reconciliation routinely do, is reported
		// on again rather than passed over. See injectionDisabledOnTemplate.
		return ctrl.Result{}, nil
	}

	if r.cfg.Mode != ModeDryRun {
		// ModeOff is the only other mode, and main.go registers no controller for it, so this is
		// unreachable in production. It is here so a Config that says off cannot report anyway.
		return ctrl.Result{}, nil
	}
	r.report(ctx, workload, pod, images, reason)
	return ctrl.Result{}, nil
}

// report emits the guard's finding on the workload. It is the guard's entire effect: no patch, no
// deletion, no state. reason carries both why the pod is broken and why the guard holds
// auto-instrumentation responsible.
func (r *PodReconciler) report(ctx context.Context, workload client.Object, pod corev1.Pod, images map[instrumentation.Type]string, reason string) {
	r.logger.Info("instrumentation guard suspects auto-instrumentation of breaking a workload",
		"pod", podName(pod), "workload", workloadName(workload), "reason", reason, "images", images)
	r.recorder.Event(workload, corev1.EventTypeWarning, ReasonInstrumentationSuspected,
		InstrumentationSuspectedMessage(podName(pod), images, reason))

	if replacesPodsItself(workload) {
		return
	}
	if isPausedDeployment(workload) {
		// The same Event reason, because the finding is the same one - a template change will not
		// reach the running pods - but the move the customer has to make is not a deletion:
		// deleting the pods of a paused Deployment just has its existing ReplicaSet recreate them
		// from the template that still injects. Naming pods here would send them after a fix that
		// does not work. See replacesPodsItself and PausedDeploymentMessage.
		r.logger.Info("instrumentation guard found a paused Deployment, so disabling injection on it will not roll out until the Deployment is resumed",
			"pod", podName(pod), "workload", workloadName(workload))
		r.recorder.Event(workload, corev1.EventTypeWarning, ReasonManualPodDeletionRequired,
			PausedDeploymentMessage(workloadName(workload), workload.GetNamespace(), workload.GetName()))
		return
	}
	// Disabling injection is the customer's move to make, and on this workload a pod template
	// change alone will not make it: the controller replaces nothing until the pods are deleted -
	// all of them, not just the one the guard found. The guard does not delete pods, so saying
	// which ones is all it can do. See replacesPodsItself.
	others, listErr := autoInstrumentedPodNames(ctx, r.client, workload)
	if listErr != nil {
		// This Event is advisory and the finding above has already been emitted, so failing the
		// reconcile here would re-emit the finding to improve a message. Name the one pod we were
		// given instead.
		r.logger.Error(listErr, "instrumentation guard could not list the workload's auto-instrumented pods, so the manual-deletion Event names only the pod it found broken",
			"pod", podName(pod), "workload", workloadName(workload))
	}
	r.logger.Info("instrumentation guard found a workload that does not replace its pods on its own, so disabling injection on it will need those pods deleted by hand",
		"pod", podName(pod), "workload", workloadName(workload), "pods", others)
	r.recorder.Event(workload, corev1.EventTypeWarning, ReasonManualPodDeletionRequired,
		ManualPodDeletionRequiredMessage(workloadName(workload), pod.Namespace, pod.Name, others))
}

// configuredImages maps each language injected into a pod to the operator's currently configured
// default image for it, for AttributeFailure to compare against. A language with no configured
// default is absent from the map, which AttributeFailure reads as "do not apply the comparison".
// It lives here rather than in attribution.go so attribution stays pure.
func configuredImages(injected map[instrumentation.Type]string) map[instrumentation.Type]string {
	configured := make(map[instrumentation.Type]string, len(injected))
	for instType := range injected {
		if image, ok := defaultImageForType(instType); ok {
			configured[instType] = image
		}
	}
	return configured
}

// SetupWithManager registers the reconciler for auto-instrumented pods only.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return o.GetLabels()[instrumentation.LabelAutoInstrumented] == "true"
		}))).
		Named("instrumentation-guard-pod").
		Complete(r)
}

func podName(pod corev1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

func workloadName(obj client.Object) string {
	return fmt.Sprintf("%T %s/%s", obj, obj.GetNamespace(), obj.GetName())
}
