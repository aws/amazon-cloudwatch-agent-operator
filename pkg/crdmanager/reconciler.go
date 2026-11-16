// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package crdmanager installs the community ServiceMonitor and PodMonitor CRDs
// (monitoring.coreos.com/v1) when they are missing, so that ServiceMonitor and
// PodMonitor scraping works with no manual step.
//
// The rules (see the "Operator-managed ServiceMonitor and PodMonitor CRDs" design):
//   - Never delete a CRD.
//   - Create a CRD only when it does not exist, stamped with ownership metadata.
//   - Change an existing CRD only when the ownership check proves the operator
//     created it and nothing else changed it since. Phase 1 (this code) never
//     changes an existing CRD; it only evaluates and reports the ownership check.
//   - Safe with more than one operator pod (for example during a rolling
//     upgrade): only a missing CRD is created, and AlreadyExists counts as success.
package crdmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Ownership metadata. These keys are a contract with customers and with later
// operator versions; do not rename them.
const (
	// FieldManager is sent on every write and recorded in metadata.managedFields.
	FieldManager = "amazon-cloudwatch-agent-operator"
	// ManagedByLabel marks a CRD the operator created. Its value is FieldManager.
	ManagedByLabel = "cloudwatch.aws.amazon.com/crd-managed-by"
	// AppliedVersionAnnotation is the upstream prometheus-operator version of the
	// schema the operator last wrote.
	AppliedVersionAnnotation = "cloudwatch.aws.amazon.com/crd-applied-version"
	// AppliedGenerationAnnotation is the metadata.generation the CRD has after the
	// operator's last write. A spec change by any other client moves
	// metadata.generation past it.
	AppliedGenerationAnnotation = "cloudwatch.aws.amazon.com/crd-applied-generation"
	// UpstreamVersionAnnotation is set by prometheus-operator in its CRD manifests
	// and identifies the live schema version, whoever wrote it.
	UpstreamVersionAnnotation = "operator.prometheus.io/version"

	// Markers other install tools leave on an object they manage. Their presence
	// means another tool now manages the CRD, even when it wrote content
	// identical to ours and so moved no spec field ownership.
	lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration" // client-side kubectl apply, Argo CD default sync
	helmReleaseAnnotation = "meta.helm.sh/release-name"                        // Helm, including --take-ownership
	appManagedByLabel     = "app.kubernetes.io/managed-by"                     // Helm and other tools

	controllerName = "prometheus-crd"

	// EventReasonNotManaged is emitted when a CRD exists that the operator did
	// not create (or that something else changed), so the operator leaves it alone.
	EventReasonNotManaged = "PrometheusCRDNotManaged"
	// EventReasonCreated is emitted when the operator creates a missing CRD.
	EventReasonCreated = "PrometheusCRDCreated"
)

var crdGVK = apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create

// Reconciler creates the embedded ServiceMonitor and PodMonitor CRDs when they
// are missing and reports, without writing, on CRDs it does not own.
type Reconciler struct {
	Client   client.Client
	Log      logr.Logger
	Recorder record.EventRecorder

	desired map[string]*apiextensionsv1.CustomResourceDefinition

	mu sync.Mutex
	// reported holds, per CRD name, the reason last logged and emitted as an
	// Event, so a CRD the operator leaves alone is reported once rather than on
	// every reconcile.
	reported map[string]string
}

// NewReconciler returns a Reconciler for the embedded CRDs.
func NewReconciler(c client.Client, log logr.Logger, recorder record.EventRecorder) (*Reconciler, error) {
	crds, err := bundledCRDs()
	if err != nil {
		return nil, err
	}
	desired := make(map[string]*apiextensionsv1.CustomResourceDefinition, len(crds))
	for _, crd := range crds {
		desired[crd.Name] = crd
	}
	return &Reconciler{
		Client:   c,
		Log:      log,
		Recorder: recorder,
		desired:  desired,
		reported: map[string]string{},
	}, nil
}

// SetupWithManager registers a controller that watches CRD metadata (not full
// schemas, so memory stays flat on clusters with many large CRDs), filtered to
// the managed names, and enqueues both names at start so a missing CRD is
// created even though no watch event exists for it.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	names := make(map[string]struct{}, len(r.desired))
	for name := range r.desired {
		names[name] = struct{}{}
	}
	isManagedName := predicate.NewPredicateFuncs(func(o client.Object) bool {
		_, ok := names[o.GetName()]
		return ok
	})
	startup := source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		for name := range names {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		}
		return nil
	})
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&apiextensionsv1.CustomResourceDefinition{}, builder.OnlyMetadata, builder.WithPredicates(isManagedName)).
		WatchesRawSource(startup).
		Complete(r)
}

// Reconcile creates the named CRD if it is missing. If it exists, Phase 1 does
// not write to it; it only reports when the ownership check fails.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	desired, ok := r.desired[req.Name]
	if !ok {
		return reconcile.Result{}, nil
	}
	log := r.Log.WithValues("crd", req.Name)

	live := &metav1.PartialObjectMetadata{}
	live.SetGroupVersionKind(crdGVK)
	err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, live)
	switch {
	case apierrors.IsNotFound(err):
		return reconcile.Result{}, r.create(ctx, log, desired)
	case err != nil:
		return reconcile.Result{}, fmt.Errorf("get CRD %s: %w", req.Name, err)
	}

	if live.GetDeletionTimestamp() != nil {
		// Being deleted. The delete event reconciles again once it is gone.
		log.V(1).Info("CRD is being deleted; waiting for removal before recreating")
		return reconcile.Result{}, nil
	}

	if owned, reason := IsOwned(live); !owned {
		r.reportOnce(log, live, reason)
		return reconcile.Result{}, nil
	}
	r.clearReported(req.Name)
	// Phase 1: the CRD is ours. Upgrading an owned CRD to a newer embedded
	// schema is Phase 2 and is intentionally not done here.
	log.V(1).Info("CRD is managed by the operator; no change", "appliedVersion", live.GetAnnotations()[AppliedVersionAnnotation])
	return reconcile.Result{}, nil
}

func (r *Reconciler) create(ctx context.Context, log logr.Logger, desired *apiextensionsv1.CustomResourceDefinition) error {
	obj := desiredWithOwnership(desired)
	err := r.Client.Create(ctx, obj, client.FieldOwner(FieldManager))
	switch {
	case apierrors.IsAlreadyExists(err):
		// Another client (or another operator pod during a rollout) created it
		// between the read and the write. The watch reconciles the new object.
		log.Info("CRD appeared before create completed; leaving it to the next reconcile")
		return nil
	case err != nil:
		return fmt.Errorf("create CRD %s: %w", desired.Name, err)
	}
	log.Info("created CRD", "version", obj.Annotations[AppliedVersionAnnotation])
	r.clearReported(desired.Name)
	r.Recorder.Eventf(obj, corev1.EventTypeNormal, EventReasonCreated,
		"Created %s at prometheus-operator version %s", desired.Name, obj.Annotations[AppliedVersionAnnotation])
	return nil
}

// desiredWithOwnership returns a copy of the embedded CRD with the ownership
// label and annotations set. A create always yields metadata.generation 1, so
// the applied generation is written in the create request itself.
func desiredWithOwnership(desired *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.CustomResourceDefinition {
	obj := desired.DeepCopy()
	obj.ResourceVersion = ""
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[ManagedByLabel] = FieldManager
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[AppliedVersionAnnotation] = obj.Annotations[UpstreamVersionAnnotation]
	obj.Annotations[AppliedGenerationAnnotation] = "1"
	return obj
}

// IsOwned reports whether the operator may treat the CRD as its own: it created
// the CRD and nothing else has changed it since the operator's last write. When
// it returns false, reason says which check failed. The checks, in order:
//  1. the crd-managed-by label is the operator's;
//  2. metadata.generation equals crd-applied-generation (no spec change since);
//  3. operator.prometheus.io/version equals crd-applied-version;
//  4. every managedFields entry that owns spec fields is the operator's;
//  5. no marker another install tool leaves is present.
func IsOwned(m metav1.Object) (bool, string) {
	labels := m.GetLabels()
	if labels[ManagedByLabel] != FieldManager {
		return false, fmt.Sprintf("label %s=%s is not set", ManagedByLabel, FieldManager)
	}
	ann := m.GetAnnotations()
	if got, want := ann[AppliedGenerationAnnotation], strconv.FormatInt(m.GetGeneration(), 10); got != want {
		return false, fmt.Sprintf("metadata.generation %s does not match %s=%q", want, AppliedGenerationAnnotation, got)
	}
	applied, upstream := ann[AppliedVersionAnnotation], ann[UpstreamVersionAnnotation]
	if applied == "" || applied != upstream {
		return false, fmt.Sprintf("%s=%q does not match %s=%q", UpstreamVersionAnnotation, upstream, AppliedVersionAnnotation, applied)
	}
	mfs := m.GetManagedFields()
	if len(mfs) == 0 {
		return false, "metadata.managedFields is empty"
	}
	for _, mf := range mfs {
		if mf.Manager == FieldManager || !ownsSpec(mf) {
			continue
		}
		return false, fmt.Sprintf("field manager %q also owns fields under spec", mf.Manager)
	}
	// A client-side apply or Helm writing content identical to ours changes no
	// spec field and no generation, so the checks above pass. The markers those
	// tools leave show that they now manage the CRD.
	for _, key := range []string{lastAppliedAnnotation, helmReleaseAnnotation} {
		if _, ok := ann[key]; ok {
			return false, fmt.Sprintf("annotation %s is set, so another tool manages this CRD", key)
		}
	}
	if _, ok := labels[appManagedByLabel]; ok {
		return false, fmt.Sprintf("label %s is set, so another tool manages this CRD", appManagedByLabel)
	}
	return true, ""
}

// ownsSpec reports whether a managedFields entry owns any field under spec.
func ownsSpec(mf metav1.ManagedFieldsEntry) bool {
	if mf.FieldsV1 == nil {
		return false
	}
	raw := mf.FieldsV1.GetRawBytes()
	if len(raw) == 0 {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		// Unreadable entry: assume it may own spec, so the operator steps back.
		return true
	}
	_, ok := fields["f:spec"]
	return ok
}

func (r *Reconciler) reportOnce(log logr.Logger, live *metav1.PartialObjectMetadata, reason string) {
	r.mu.Lock()
	already := r.reported[live.Name] == reason
	r.reported[live.Name] = reason
	r.mu.Unlock()
	if already {
		return
	}
	log.Info("CRD exists and is not managed by the operator; leaving it unchanged", "reason", reason)
	r.Recorder.Eventf(live, corev1.EventTypeNormal, EventReasonNotManaged,
		"Leaving %s unchanged: %s", live.Name, reason)
}

func (r *Reconciler) clearReported(name string) {
	r.mu.Lock()
	delete(r.reported, name)
	r.mu.Unlock()
}
