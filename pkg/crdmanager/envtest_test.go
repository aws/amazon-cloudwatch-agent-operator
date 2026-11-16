// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crdmanager

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// These tests run the reconciler and IsOwned against a real API server
// (envtest). They cover the design's E1-E8 cases.

var managedNames = []string{smName, pmName}

// countingClient wraps envClient and counts every write the reconciler makes.
func countingClient(writes *int) client.WithWatch {
	return interceptor.NewClient(envClient, interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			*writes++
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			*writes++
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
			*writes++
			return c.Patch(ctx, obj, p, opts...)
		},
		Apply: func(ctx context.Context, c client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
			*writes++
			return c.Apply(ctx, obj, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			*writes++
			return c.Delete(ctx, obj, opts...)
		},
	})
}

// resetCRDs deletes both managed CRDs, if present, and waits until they are gone.
func resetCRDs(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, name := range managedNames {
		crd := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err := envClient.Delete(ctx, crd); err != nil && !apierrors.IsNotFound(err) {
			require.NoError(t, err)
		}
	}
	require.Eventually(t, func() bool {
		for _, name := range managedNames {
			err := envClient.Get(ctx, client.ObjectKey{Name: name}, &apiextensionsv1.CustomResourceDefinition{})
			if !apierrors.IsNotFound(err) {
				return false
			}
		}
		return true
	}, 30*time.Second, 200*time.Millisecond, "CRDs were not deleted")
}

// reconcileAll runs one reconcile per managed CRD.
func reconcileAll(t *testing.T, r *Reconciler) {
	t.Helper()
	for _, name := range managedNames {
		_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		require.NoError(t, err)
	}
}

func liveCRD(t *testing.T, name string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, envClient.Get(context.Background(), client.ObjectKey{Name: name}, crd))
	return crd
}

// settledCRD waits until the API server has finished its own status writes to
// the CRD (NamesAccepted, then Established) and returns it. Those writes move
// resourceVersion after any create or spec change, so a test comparing
// resourceVersion must start from a settled CRD.
func settledCRD(t *testing.T, name string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	var crd *apiextensionsv1.CustomResourceDefinition
	require.Eventually(t, func() bool {
		first := liveCRD(t, name)
		if !isEstablished(first) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
		crd = liveCRD(t, name)
		return crd.ResourceVersion == first.ResourceVersion
	}, 30*time.Second, 100*time.Millisecond, "%s did not settle", name)
	return crd
}

func isEstablished(crd *apiextensionsv1.CustomResourceDefinition) bool {
	for _, c := range crd.Status.Conditions {
		if c.Type == apiextensionsv1.Established {
			return c.Status == apiextensionsv1.ConditionTrue
		}
	}
	return false
}

// createdByOperator resets the cluster, lets the reconciler create both CRDs and
// returns the live ServiceMonitor CRD.
func createdByOperator(t *testing.T) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	resetCRDs(t)
	r, err := NewReconciler(envClient, logr.Discard(), record.NewFakeRecorder(10))
	require.NoError(t, err)
	reconcileAll(t, r)
	crd := liveCRD(t, smName)
	owned, reason := IsOwned(crd)
	require.True(t, owned, "precondition: a CRD the operator created must be owned: %s", reason)
	return crd
}

// specManagers returns the managers whose managedFields entry owns spec fields.
func specManagers(crd *apiextensionsv1.CustomResourceDefinition) []string {
	var out []string
	for _, mf := range crd.GetManagedFields() {
		if ownsSpec(mf) {
			out = append(out, mf.Manager)
		}
	}
	return out
}

// assertStepsBack checks E6: once the ownership check fails, repeated
// reconciles make zero writes and emit exactly one PrometheusCRDNotManaged Event.
func assertStepsBack(t *testing.T, wantReason string) {
	t.Helper()
	before := settledCRD(t, smName).ResourceVersion

	writes := 0
	rec := record.NewFakeRecorder(10)
	r, err := NewReconciler(countingClient(&writes), logr.Discard(), rec)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: smName}})
		require.NoError(t, err)
	}

	assert.Zero(t, writes, "the operator wrote to a CRD it does not own")
	assert.Equal(t, before, liveCRD(t, smName).ResourceVersion, "the CRD changed")
	events := drain(rec)
	require.Len(t, events, 1, "step-back must be reported exactly once")
	assert.Contains(t, events[0], EventReasonNotManaged)
	assert.Contains(t, events[0], wantReason)
}

// E1
func TestEnvtestCreatesBothCRDsOwned(t *testing.T) {
	resetCRDs(t)
	rec := record.NewFakeRecorder(10)
	r, err := NewReconciler(envClient, logr.Discard(), rec)
	require.NoError(t, err)
	reconcileAll(t, r)

	for _, name := range managedNames {
		crd := liveCRD(t, name)
		assert.Equal(t, int64(1), crd.Generation, "%s generation", name)
		assert.Equal(t, []string{FieldManager}, specManagers(crd), "%s spec managers", name)
		owned, reason := IsOwned(crd)
		assert.True(t, owned, "%s: %s", name, reason)
	}
	events := drain(rec)
	require.Len(t, events, 2)
	for _, e := range events {
		assert.Contains(t, e, EventReasonCreated)
	}
}

// E2
func TestEnvtestLabelOnlyChangeKeepsOwnership(t *testing.T) {
	crd := createdByOperator(t)
	patch := client.MergeFrom(crd.DeepCopy())
	crd.Labels["team"] = "observability"
	require.NoError(t, envClient.Patch(context.Background(), crd, patch, client.FieldOwner("kubectl-label")))

	live := liveCRD(t, smName)
	assert.Equal(t, int64(1), live.Generation, "a label change must not move generation")
	owned, reason := IsOwned(live)
	assert.True(t, owned, reason)
}

// E3 and E6
func TestEnvtestForeignServerSideApplyStepsBack(t *testing.T) {
	createdByOperator(t)
	desired := desiredWithOwnership(desiredCRD(t, smName))
	u := applyConfig(t, desired)
	require.NoError(t, envClient.Apply(context.Background(), client.ApplyConfigurationFromUnstructured(u),
		client.FieldOwner("other-tool"), client.ForceOwnership))

	live := liveCRD(t, smName)
	assert.Contains(t, specManagers(live), "other-tool")
	owned, reason := IsOwned(live)
	assert.False(t, owned)
	assert.Contains(t, reason, `"other-tool"`)
	assertStepsBack(t, `"other-tool"`)
}

// E4 and E6
func TestEnvtestForeignSpecChangeStepsBack(t *testing.T) {
	createdByOperator(t)
	updateCRD(t, "other-client", func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Description = "edited by another client"
	})

	live := liveCRD(t, smName)
	assert.Equal(t, int64(2), live.Generation, "a spec change must move generation")
	owned, reason := IsOwned(live)
	assert.False(t, owned)
	assert.Contains(t, reason, "metadata.generation")
	assertStepsBack(t, "metadata.generation")
}

// E5 and E6
func TestEnvtestNewerUpstreamVersionStepsBack(t *testing.T) {
	crd := createdByOperator(t)
	patch := client.MergeFrom(crd.DeepCopy())
	crd.Annotations[UpstreamVersionAnnotation] = "0.94.2"
	require.NoError(t, envClient.Patch(context.Background(), crd, patch, client.FieldOwner("other-client")))

	owned, reason := IsOwned(liveCRD(t, smName))
	assert.False(t, owned)
	assert.Contains(t, reason, UpstreamVersionAnnotation)
	assertStepsBack(t, UpstreamVersionAnnotation)
}

// E7
func TestEnvtestDeletedCRDIsRecreatedOwned(t *testing.T) {
	first := createdByOperator(t)
	require.NoError(t, envClient.Delete(context.Background(), first))
	require.Eventually(t, func() bool {
		err := envClient.Get(context.Background(), client.ObjectKey{Name: smName}, &apiextensionsv1.CustomResourceDefinition{})
		return apierrors.IsNotFound(err)
	}, 30*time.Second, 200*time.Millisecond)

	r, err := NewReconciler(envClient, logr.Discard(), record.NewFakeRecorder(10))
	require.NoError(t, err)
	reconcileAll(t, r)

	again := liveCRD(t, smName)
	assert.NotEqual(t, first.UID, again.UID, "expected a new object")
	assert.Equal(t, int64(1), again.Generation)
	assert.Equal(t, "1", again.Annotations[AppliedGenerationAnnotation])
	owned, reason := IsOwned(again)
	assert.True(t, owned, reason)
}

// E8 and E6: a client-side kubectl apply with content identical to ours moves no
// spec field ownership and no generation, so only check 5 catches it.
func TestEnvtestClientSideApplyMarkerStepsBack(t *testing.T) {
	createdByOperator(t)
	lastApplied, err := json.Marshal(desiredWithOwnership(desiredCRD(t, smName)))
	require.NoError(t, err)
	updateCRD(t, "kubectl-client-side-apply", func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Annotations[lastAppliedAnnotation] = string(lastApplied)
	})

	live := liveCRD(t, smName)
	assert.Equal(t, int64(1), live.Generation, "identical content must not move generation")
	assert.Equal(t, []string{FieldManager}, specManagers(live), "identical content must not add a spec manager")
	owned, reason := IsOwned(live)
	assert.False(t, owned)
	assert.Contains(t, reason, lastAppliedAnnotation)
	assertStepsBack(t, lastAppliedAnnotation)
}

// E8 and E6: Helm taking ownership (helm --take-ownership) adds its release
// annotations and managed-by label without changing spec.
func TestEnvtestHelmOwnershipMarkersStepBack(t *testing.T) {
	crd := createdByOperator(t)
	patch := client.MergeFrom(crd.DeepCopy())
	crd.Annotations[helmReleaseAnnotation] = "kube-prometheus-stack"
	crd.Annotations["meta.helm.sh/release-namespace"] = "monitoring"
	crd.Labels[appManagedByLabel] = "Helm"
	require.NoError(t, envClient.Patch(context.Background(), crd, patch, client.FieldOwner("helm")))

	live := liveCRD(t, smName)
	assert.Equal(t, int64(1), live.Generation)
	assert.Equal(t, []string{FieldManager}, specManagers(live))
	owned, reason := IsOwned(live)
	assert.False(t, owned)
	assert.Contains(t, reason, helmReleaseAnnotation)
	assertStepsBack(t, helmReleaseAnnotation)
}

// updateCRD re-reads the ServiceMonitor CRD, applies mutate and updates it as
// manager, retrying when the API server changed it in between (it sets status
// conditions right after a create).
func updateCRD(t *testing.T, manager string, mutate func(*apiextensionsv1.CustomResourceDefinition)) {
	t.Helper()
	require.NoError(t, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := envClient.Get(context.Background(), client.ObjectKey{Name: smName}, crd); err != nil {
			return err
		}
		mutate(crd)
		return envClient.Update(context.Background(), crd, client.FieldOwner(manager))
	}))
}

func desiredCRD(t *testing.T, name string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	crds, err := bundledCRDs()
	require.NoError(t, err)
	for _, crd := range crds {
		if crd.Name == name {
			return crd
		}
	}
	t.Fatalf("no embedded CRD %s", name)
	return nil
}

// applyConfig converts a CRD into an unstructured apply configuration with only
// the fields a manifest would carry.
func applyConfig(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition) *unstructured.Unstructured {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
	require.NoError(t, err)
	u := &unstructured.Unstructured{Object: obj}
	u.SetAPIVersion(apiextensionsv1.SchemeGroupVersion.String())
	u.SetKind("CustomResourceDefinition")
	unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(u.Object, "status")
	for k := range u.GetAnnotations() {
		if strings.HasPrefix(k, "cloudwatch.aws.amazon.com/") {
			// The other tool's manifest is upstream's, without our annotations.
			ann := u.GetAnnotations()
			delete(ann, k)
			u.SetAnnotations(ann)
		}
	}
	return u
}
