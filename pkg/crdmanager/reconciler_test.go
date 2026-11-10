// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crdmanager

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	smName = "servicemonitors.monitoring.coreos.com"
	pmName = "podmonitors.monitoring.coreos.com"
)

func specFields() *metav1.FieldsV1 {
	return metav1.NewFieldsV1(`{"f:metadata":{"f:labels":{}},"f:spec":{"f:group":{}}}`)
}

func statusFields() *metav1.FieldsV1 {
	return metav1.NewFieldsV1(`{"f:status":{"f:conditions":{}}}`)
}

// ownedMeta returns metadata that passes every ownership check.
func ownedMeta() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:       smName,
		Generation: 1,
		Labels:     map[string]string{ManagedByLabel: FieldManager},
		Annotations: map[string]string{
			UpstreamVersionAnnotation:   BundledVersion,
			AppliedVersionAnnotation:    BundledVersion,
			AppliedGenerationAnnotation: "1",
		},
		ManagedFields: []metav1.ManagedFieldsEntry{
			{Manager: FieldManager, Operation: metav1.ManagedFieldsOperationUpdate, FieldsV1: specFields()},
			{Manager: "kube-apiserver", Operation: metav1.ManagedFieldsOperationUpdate, Subresource: "status", FieldsV1: statusFields()},
		},
	}
}

func TestIsOwned(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(m *metav1.ObjectMeta)
		wantOwned  bool
		wantReason string
	}{
		{name: "all checks pass", mutate: func(*metav1.ObjectMeta) {}, wantOwned: true},
		{
			name:       "label missing (installed by another tool)",
			mutate:     func(m *metav1.ObjectMeta) { m.Labels = nil },
			wantReason: "label",
		},
		{
			name:       "label has another value",
			mutate:     func(m *metav1.ObjectMeta) { m.Labels[ManagedByLabel] = "helm" },
			wantReason: "label",
		},
		{
			name:       "generation moved past the applied generation (spec changed by another client)",
			mutate:     func(m *metav1.ObjectMeta) { m.Generation = 2 },
			wantReason: "metadata.generation",
		},
		{
			name:       "applied generation annotation missing",
			mutate:     func(m *metav1.ObjectMeta) { delete(m.Annotations, AppliedGenerationAnnotation) },
			wantReason: "metadata.generation",
		},
		{
			name:       "upstream version differs from applied version (newer bundle applied over ours)",
			mutate:     func(m *metav1.ObjectMeta) { m.Annotations[UpstreamVersionAnnotation] = "0.93.0" },
			wantReason: UpstreamVersionAnnotation,
		},
		{
			name: "applied version annotation missing",
			mutate: func(m *metav1.ObjectMeta) {
				delete(m.Annotations, AppliedVersionAnnotation)
			},
			wantReason: UpstreamVersionAnnotation,
		},
		{
			name:       "managedFields empty",
			mutate:     func(m *metav1.ObjectMeta) { m.ManagedFields = nil },
			wantReason: "managedFields is empty",
		},
		{
			name: "second manager owns spec (server-side apply by another client)",
			mutate: func(m *metav1.ObjectMeta) {
				m.ManagedFields = append(m.ManagedFields, metav1.ManagedFieldsEntry{
					Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply, FieldsV1: specFields(),
				})
			},
			wantReason: `"kubectl"`,
		},
		{
			name: "recreated from a saved copy of ours by another client",
			mutate: func(m *metav1.ObjectMeta) {
				m.ManagedFields[0].Manager = "kubectl-create"
			},
			wantReason: `"kubectl-create"`,
		},
		{
			name: "another manager touching only metadata does not fail the check",
			mutate: func(m *metav1.ObjectMeta) {
				m.ManagedFields = append(m.ManagedFields, metav1.ManagedFieldsEntry{
					Manager: "kubectl-label", Operation: metav1.ManagedFieldsOperationUpdate,
					FieldsV1: metav1.NewFieldsV1(`{"f:metadata":{"f:labels":{"f:team":{}}}}`),
				})
			},
			wantOwned: true,
		},
		{
			name: "unreadable managedFields entry is treated as owning spec",
			mutate: func(m *metav1.ObjectMeta) {
				m.ManagedFields = append(m.ManagedFields, metav1.ManagedFieldsEntry{
					Manager: "other", FieldsV1: metav1.NewFieldsV1(`not-json`),
				})
			},
			wantReason: `"other"`,
		},
		{
			name:       "client-side kubectl apply marker (identical content, no spec manager)",
			mutate:     func(m *metav1.ObjectMeta) { m.Annotations[lastAppliedAnnotation] = "{}" },
			wantReason: lastAppliedAnnotation,
		},
		{
			name:       "Helm release marker (helm --take-ownership)",
			mutate:     func(m *metav1.ObjectMeta) { m.Annotations[helmReleaseAnnotation] = "kps" },
			wantReason: helmReleaseAnnotation,
		},
		{
			name:       "app.kubernetes.io/managed-by label from another tool",
			mutate:     func(m *metav1.ObjectMeta) { m.Labels[appManagedByLabel] = "Helm" },
			wantReason: appManagedByLabel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ownedMeta()
			tt.mutate(m)
			owned, reason := IsOwned(m)
			assert.Equal(t, tt.wantOwned, owned, reason)
			if !tt.wantOwned {
				assert.Contains(t, reason, tt.wantReason)
			}
		})
	}
}

func TestBundledCRDs(t *testing.T) {
	crds, err := bundledCRDs()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, crd := range crds {
		names[crd.Name] = true
		assert.Equal(t, "monitoring.coreos.com", crd.Spec.Group)
		assert.Equal(t, BundledVersion, crd.Annotations[UpstreamVersionAnnotation],
			"embedded %s is not at BundledVersion", crd.Name)
		require.Len(t, crd.Spec.Versions, 1)
		assert.Equal(t, "v1", crd.Spec.Versions[0].Name)
	}
	assert.Equal(t, map[string]bool{smName: true, pmName: true}, names)
}

// The embedded schema must match the prometheus-operator client the operator
// and target allocator are built with, so the operator never installs a schema
// the target allocator cannot read.
func TestBundledVersionMatchesGoMod(t *testing.T) {
	raw, err := os.ReadFile("../../go.mod")
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^\s*github\.com/prometheus-operator/prometheus-operator v(\S+)`).FindSubmatch(raw)
	require.NotNil(t, m, "prometheus-operator not found in go.mod")
	assert.Equal(t, BundledVersion, string(m[1]))
}

func TestDesiredWithOwnership(t *testing.T) {
	crds, err := bundledCRDs()
	require.NoError(t, err)
	orig := crds[0]
	got := desiredWithOwnership(orig)

	assert.Equal(t, FieldManager, got.Labels[ManagedByLabel])
	assert.Equal(t, BundledVersion, got.Annotations[AppliedVersionAnnotation])
	assert.Equal(t, "1", got.Annotations[AppliedGenerationAnnotation])
	assert.Equal(t, BundledVersion, got.Annotations[UpstreamVersionAnnotation])
	assert.Empty(t, got.OwnerReferences, "operator-created CRDs must have no owner reference")
	// The embedded object is not modified.
	assert.NotContains(t, orig.Annotations, AppliedVersionAnnotation)
	assert.NotContains(t, orig.Labels, ManagedByLabel)
}

func newTestReconciler(t *testing.T, c client.Client) (*Reconciler, *record.FakeRecorder) {
	t.Helper()
	rec := record.NewFakeRecorder(10)
	r, err := NewReconciler(c, logr.Discard(), rec)
	require.NoError(t, err)
	return r, rec
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(s))
	return s
}

func reconcileName(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
}

func drain(rec *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

func TestReconcileCreatesMissingCRDs(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	r, rec := newTestReconciler(t, c)

	for _, name := range []string{smName, pmName} {
		reconcileName(t, r, name)

		got := &apiextensionsv1.CustomResourceDefinition{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, got))
		assert.Equal(t, FieldManager, got.Labels[ManagedByLabel])
		assert.Equal(t, BundledVersion, got.Annotations[AppliedVersionAnnotation])
		assert.Equal(t, "1", got.Annotations[AppliedGenerationAnnotation])
		assert.Empty(t, got.OwnerReferences)
	}
	events := drain(rec)
	require.Len(t, events, 2)
	for _, e := range events {
		assert.Contains(t, e, EventReasonCreated)
	}
}

func TestReconcileLeavesUnmanagedCRDUnchanged(t *testing.T) {
	foreign := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        smName,
			Annotations: map[string]string{UpstreamVersionAnnotation: "0.95.0"},
		},
	}
	writes := 0
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(foreign).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes++
				return cl.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				writes++
				return cl.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
				writes++
				return cl.Patch(ctx, obj, p, opts...)
			},
			Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				writes++
				return cl.Delete(ctx, obj, opts...)
			},
		}).Build()
	r, rec := newTestReconciler(t, c)

	// Reconcile several times: no write ever, and the step-back is reported once.
	for i := 0; i < 3; i++ {
		reconcileName(t, r, smName)
	}
	assert.Zero(t, writes, "operator must not write to a CRD it does not own")

	got := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: smName}, got))
	assert.Equal(t, "0.95.0", got.Annotations[UpstreamVersionAnnotation], "no downgrade")
	assert.NotContains(t, got.Labels, ManagedByLabel)

	events := drain(rec)
	require.Len(t, events, 1, "step-back is reported once, not per reconcile")
	assert.Contains(t, events[0], EventReasonNotManaged)
}

func TestReconcileAlreadyExistsOnCreateIsNotAnError(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
				return apierrors.NewAlreadyExists(apiextensionsv1.Resource("customresourcedefinitions"), smName)
			},
		}).Build()
	r, rec := newTestReconciler(t, c)
	reconcileName(t, r, smName)
	assert.Empty(t, drain(rec))
}

func TestReconcileCreateSendsFieldManager(t *testing.T) {
	var owner string
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				co := &client.CreateOptions{}
				co.ApplyOptions(opts)
				owner = co.FieldManager
				return cl.Create(ctx, obj, opts...)
			},
		}).Build()
	r, _ := newTestReconciler(t, c)
	reconcileName(t, r, pmName)
	assert.Equal(t, FieldManager, owner)
}

func TestReconcileIgnoresOtherCRDs(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	r, rec := newTestReconciler(t, c)
	reconcileName(t, r, "prometheuses.monitoring.coreos.com")
	list := &apiextensionsv1.CustomResourceDefinitionList{}
	require.NoError(t, c.List(context.Background(), list))
	assert.Empty(t, list.Items)
	assert.Empty(t, drain(rec))
}

func TestReconcileSkipsCRDBeingDeleted(t *testing.T) {
	now := metav1.Now()
	deleting := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: smName, DeletionTimestamp: &now, Finalizers: []string{"customresourcecleanup.apiextensions.k8s.io"}},
	}
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(deleting).Build()
	r, rec := newTestReconciler(t, c)
	reconcileName(t, r, smName)
	assert.Empty(t, drain(rec), "no report while the CRD is terminating")
}

// The operator never asks for the delete verb on CRDs.
func TestRBACMarkerHasNoDelete(t *testing.T) {
	raw, err := os.ReadFile("reconciler.go")
	require.NoError(t, err)
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "+kubebuilder:rbac:groups=apiextensions.k8s.io") {
			assert.NotContains(t, line, "delete")
			assert.NotContains(t, line, "update")
			assert.NotContains(t, line, "patch")
			return
		}
	}
	t.Fatal("apiextensions RBAC marker not found")
}
