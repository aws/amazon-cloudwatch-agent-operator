// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"context"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// ResolveWorkload returns the Deployment, StatefulSet, or DaemonSet that owns the pod, or
// (nil, nil) when the pod has no owner the guard can report on: a bare pod, a Job or CronJob pod, a
// ReplicaSet with no Deployment above it, any other kind, and a workload that has since been
// deleted. Only a real API error is returned as an error.
func ResolveWorkload(ctx context.Context, r client.Reader, pod corev1.Pod) (client.Object, error) {
	for _, owner := range ownersToWalk(pod.OwnerReferences, metav1.GetControllerOf(&pod)) {
		if !isAppsV1(owner) {
			// A same-named kind from another API group is a different object entirely, and the
			// guard addresses its Events to what it resolves, so it must not fall through to
			// apps/v1 and blame an object it never looked at.
			continue
		}
		switch strings.ToLower(owner.Kind) {
		case "replicaset":
			replicaSet := &appsv1.ReplicaSet{}
			found, err := getWithRetry(ctx, r, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, replicaSet)
			if err != nil || !found || !isNamedOwner(owner, replicaSet) {
				return nil, err
			}
			return resolveReplicaSetOwner(ctx, r, replicaSet)
		case "statefulset":
			statefulSet := &appsv1.StatefulSet{}
			found, err := getWithRetry(ctx, r, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, statefulSet)
			if err != nil || !found || !isNamedOwner(owner, statefulSet) {
				return nil, err
			}
			return statefulSet, nil
		case "daemonset":
			daemonSet := &appsv1.DaemonSet{}
			found, err := getWithRetry(ctx, r, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, daemonSet)
			if err != nil || !found || !isNamedOwner(owner, daemonSet) {
				return nil, err
			}
			return daemonSet, nil
		}
	}
	return nil, nil
}

// resolveReplicaSetOwner follows a ReplicaSet to its Deployment. A ReplicaSet with no Deployment
// owner is not something the guard can report against, so it resolves to nil.
func resolveReplicaSetOwner(ctx context.Context, r client.Reader, replicaSet *appsv1.ReplicaSet) (client.Object, error) {
	for _, owner := range ownersToWalk(replicaSet.OwnerReferences, metav1.GetControllerOf(replicaSet)) {
		if !isAppsV1(owner) || strings.ToLower(owner.Kind) != "deployment" {
			continue
		}
		deployment := &appsv1.Deployment{}
		found, err := getWithRetry(ctx, r, types.NamespacedName{Namespace: replicaSet.Namespace, Name: owner.Name}, deployment)
		if err != nil || !found || !isNamedOwner(owner, deployment) {
			return nil, err
		}
		return deployment, nil
	}
	return nil, nil
}

// autoInstrumentedPodNames returns the names of the auto-instrumented pods the workload controls
// itself, for the manual-deletion Event to name. On a workload that does not replace its own pods
// every one of them has to be deleted by hand once injection is disabled, so naming only the pod
// the guard found broken tells the customer to fix one pod out of however many are affected.
//
// The List is restricted to the workload's namespace and to the LabelAutoInstrumented label, which
// is the same label main.go's cache.Options.ByObject selector already restricts the manager's pod
// cache to: the cached client answers this from pods it is holding anyway, so the call costs no
// API request and needs no RBAC beyond the `pods get;list;watch` pod_reconciler.go's markers
// already ask for.
//
// A pod is kept when its CONTROLLER owner reference points at this workload, by UID. The reasoning
// is ownersToWalk's: an object has at most one controller reference, and that is the one that
// created it from the workload's pod template, so a plain owner reference is not evidence that
// deleting the pod will produce a replacement. The UID rather than the name is what distinguishes
// this workload from a deleted workload of the same name whose pods have outlived it.
//
// This is only correct for a workload that owns its pods DIRECTLY. A Deployment does not: its pods
// are controlled by a ReplicaSet, so this would return none of them. That is not a problem,
// because the only caller is the manual-deletion path, which replacesPodsItself restricts to
// StatefulSets and OnDelete DaemonSets - both of which do own their pods directly.
func autoInstrumentedPodNames(ctx context.Context, r client.Reader, workload client.Object) ([]string, error) {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(workload.GetNamespace()),
		client.MatchingLabels{instrumentation.LabelAutoInstrumented: "true"},
	); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(pods.Items))
	for i := range pods.Items {
		controller := metav1.GetControllerOf(&pods.Items[i])
		if controller == nil || controller.UID != workload.GetUID() {
			continue
		}
		names = append(names, pods.Items[i].Name)
	}
	return names, nil
}

// ownersToWalk returns the owner references to consider, in order. An object has at most one
// controller reference, and that is the one that owns its pod template, so when there is one it is
// the only candidate; without one, every reference is considered as before.
func ownersToWalk(owners []metav1.OwnerReference, controller *metav1.OwnerReference) []metav1.OwnerReference {
	if controller != nil {
		return []metav1.OwnerReference{*controller}
	}
	return owners
}

// isAppsV1 reports whether the owner reference points into the apps/v1 group, the only group whose
// Deployments, StatefulSets, and DaemonSets the guard knows anything about.
func isAppsV1(owner metav1.OwnerReference) bool {
	return owner.APIVersion == appsv1.SchemeGroupVersion.String()
}

// isNamedOwner reports whether the object just fetched BY NAME is the object the owner reference
// actually names, which is a question of UID and not of name.
//
// A name is reusable and an owner reference outlives what it points at. An old ReplicaSet whose
// Deployment has been deleted keeps its ownerReferences, so a Deployment created afterwards with
// the same name resolves from them - and the guard would then address its Event to a workload that
// never owned the pod it is reporting on. The same holds one hop down for a pod whose StatefulSet
// or DaemonSet was deleted and recreated. A mismatch is read as the owner being gone, which is
// what it is, and the walk stops: see ResolveWorkload's contract for a deleted workload.
func isNamedOwner(owner metav1.OwnerReference, obj client.Object) bool {
	return owner.UID == obj.GetUID()
}

// podTemplate returns the workload's pod template, or nil for a kind that has none the guard
// knows about. The guard only READS it - to see whether the inject annotations the customer would
// change are still set to "false". See injectionDisabledOnTemplate.
func podTemplate(obj client.Object) *corev1.PodTemplateSpec {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return &o.Spec.Template
	case *appsv1.StatefulSet:
		return &o.Spec.Template
	case *appsv1.DaemonSet:
		return &o.Spec.Template
	default:
		return nil
	}
}

// isPausedDeployment reports whether the workload is a Deployment with spec.paused set. See
// replacesPodsItself for why that matters and PausedDeploymentMessage for what the guard says
// about it.
func isPausedDeployment(obj client.Object) bool {
	deployment, ok := obj.(*appsv1.Deployment)
	return ok && deployment.Spec.Paused
}

// workloadGetBackOff is the retry schedule for the owner walk, the same one
// sdkInjector.addParentResourceLabels uses, because a single Get fails occasionally. It is a
// variable so tests can shorten it.
var workloadGetBackOff = wait.Backoff{Duration: 10 * time.Millisecond, Factor: 1.5, Jitter: 0.1, Steps: 20, Cap: 2 * time.Second}

// getWithRetry fetches an object, retrying a NotFound. It reports whether the object was found; a
// NotFound that survives the retry is not an error, because an object that is gone is nothing the
// guard needs to report on.
func getWithRetry(ctx context.Context, r client.Reader, name types.NamespacedName, obj client.Object) (bool, error) {
	backOff := workloadGetBackOff
	err := retry.OnError(backOff, apierrors.IsNotFound, func() error {
		return r.Get(ctx, name, obj)
	})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// replacesPodsItself reports whether changing a workload's pod template is enough for the change
// to reach the running pods. When it is not, a customer who disables injection on the template is
// left with the broken pods still running, so the guard says so in a second Event; it never
// deletes a pod itself. That Event names every auto-instrumented pod of the workload, bounded by
// maxNamedPods, because on an OnDelete workload the controller replaces nothing until each of them
// is deleted - see ManualPodDeletionRequiredMessage and autoInstrumentedPodNames.
//
// MEASURED on a live cluster at Kubernetes v1.34.8, one broken auto-instrumented pod per workload:
//
//	Deployment,  RollingUpdate (default)  recovers
//	Deployment,  Recreate                 recovers
//	DaemonSet,   RollingUpdate (default)  recovers
//	DaemonSet,   OnDelete                 does NOT
//	StatefulSet, RollingUpdate (default)  does NOT
//	StatefulSet, OnDelete                 does NOT
//
// A StatefulSet does not self-heal even on its DEFAULT update strategy, which is the surprise
// here. Its default pod management policy is OrderedReady, so the controller replaces a pod and
// then waits for it to become Ready before touching the next one. The pod the guard is reacting
// to never becomes Ready - that is what being broken means - so the rollout stops on it and the
// new template is never applied. A partitioned RollingUpdate has the same shape for any pod below
// the partition.
//
// A PAUSED Deployment is the exception to the Deployment rows, and it is read from the spec rather
// than taken from the table above. spec.paused stops the Deployment controller acting on the spec
// at all, so a template change creates no new ReplicaSet and the broken pods keep running. Nor
// does deleting them help: the existing ReplicaSet recreates them from the template it already
// has, which is the one that still injects. The customer has to resume the Deployment, which is
// what PausedDeploymentMessage asks for instead of a deletion.
//
// The table is encoded here rather than probed at runtime. A future change to OrderedReady
// semantics would make it stale, which is why the measurement is written down.
func replacesPodsItself(obj client.Object) bool {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return !o.Spec.Paused
	case *appsv1.DaemonSet:
		return o.Spec.UpdateStrategy.Type != appsv1.OnDeleteDaemonSetStrategyType
	case *appsv1.StatefulSet:
		return false
	default:
		return false
	}
}
