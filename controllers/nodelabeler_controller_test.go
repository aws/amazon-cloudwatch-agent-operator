// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func newTestNodeLabelerReconciler(objs ...client.Object) *NodeLabelerReconciler {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &NodeLabelerReconciler{
		Client: c,
		Scheme: scheme,
		Log:    logf.Log.WithName("nodelabeler-test"),
	}
}

func node(name string, labels map[string]string, capacity corev1.ResourceList) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status:     corev1.NodeStatus{Capacity: capacity},
	}
}

func reconcileAndGet(t *testing.T, r *NodeLabelerReconciler, name string) *corev1.Node {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
	var got corev1.Node
	require.NoError(t, r.Get(context.Background(), types.NamespacedName{Name: name}, &got))
	return &got
}

// (a) node with nvidia.com/gpu capacity -> gpu label added, neuron label absent.
func TestNodeLabeler_GPUCapacityAddsLabel(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("gpu-node", nil, corev1.ResourceList{
		nvidiaGPUResource:  resource.MustParse("1"),
		corev1.ResourceCPU: resource.MustParse("8"),
	}))
	got := reconcileAndGet(t, r, "gpu-node")
	assert.Equal(t, "true", got.Labels[GPUPresentLabel])
	_, hasNeuron := got.Labels[NeuronPresentLabel]
	assert.False(t, hasNeuron)
}

// Presence-of-key semantics: a GPU node whose device plugin died keeps the
// capacity key at quantity 0 and must still be labeled.
func TestNodeLabeler_GPUCapacityZeroQuantityStillLabels(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("gpu-unhealthy", nil, corev1.ResourceList{
		nvidiaGPUResource: resource.MustParse("0"),
	}))
	got := reconcileAndGet(t, r, "gpu-unhealthy")
	assert.Equal(t, "true", got.Labels[GPUPresentLabel])
}

// (b) node with no relevant capacity -> no managed labels.
func TestNodeLabeler_NoRelevantCapacityNoLabels(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("plain-node", nil, corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("4"),
		corev1.ResourceMemory: resource.MustParse("16Gi"),
	}))
	got := reconcileAndGet(t, r, "plain-node")
	_, hasGPU := got.Labels[GPUPresentLabel]
	_, hasNeuron := got.Labels[NeuronPresentLabel]
	assert.False(t, hasGPU)
	assert.False(t, hasNeuron)
}

// (c) node carrying our gpu label but capacity key gone -> label removed.
func TestNodeLabeler_StaleGPULabelRemoved(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("was-gpu", map[string]string{
		GPUPresentLabel: "true",
	}, corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("4"),
	}))
	got := reconcileAndGet(t, r, "was-gpu")
	_, hasGPU := got.Labels[GPUPresentLabel]
	assert.False(t, hasGPU)
}

// (d) node with a neuron capacity key -> neuron label added.
func TestNodeLabeler_NeuronCapacityAddsLabel(t *testing.T) {
	for _, res := range []corev1.ResourceName{neuronResource, neuronCoreResource, neuronDeviceResource} {
		r := newTestNodeLabelerReconciler(node("neuron-node", nil, corev1.ResourceList{
			res: resource.MustParse("1"),
		}))
		got := reconcileAndGet(t, r, "neuron-node")
		assert.Equal(t, "true", got.Labels[NeuronPresentLabel], "resource %s should set neuron label", res)
		_, hasGPU := got.Labels[GPUPresentLabel]
		assert.False(t, hasGPU)
	}
}

// (e) already-correct node -> reconcile is a no-op: no error, labels unchanged,
// and no API write (resourceVersion unchanged).
func TestNodeLabeler_AlreadyCorrectIsNoOp(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("gpu-labeled", map[string]string{
		GPUPresentLabel: "true",
	}, corev1.ResourceList{
		nvidiaGPUResource: resource.MustParse("1"),
	}))
	var before corev1.Node
	require.NoError(t, r.Get(context.Background(), types.NamespacedName{Name: "gpu-labeled"}, &before))

	got := reconcileAndGet(t, r, "gpu-labeled")
	assert.Equal(t, "true", got.Labels[GPUPresentLabel])
	assert.Equal(t, before.ResourceVersion, got.ResourceVersion, "no-op reconcile must not write")
}

// Other existing labels must survive the metadata.labels merge patch.
func TestNodeLabeler_PreservesOtherLabels(t *testing.T) {
	r := newTestNodeLabelerReconciler(node("mixed", map[string]string{
		"kubernetes.io/hostname":      "mixed",
		"node.kubernetes.io/instance": "keep-me",
	}, corev1.ResourceList{
		nvidiaGPUResource: resource.MustParse("1"),
	}))
	got := reconcileAndGet(t, r, "mixed")
	assert.Equal(t, "true", got.Labels[GPUPresentLabel])
	assert.Equal(t, "mixed", got.Labels["kubernetes.io/hostname"])
	assert.Equal(t, "keep-me", got.Labels["node.kubernetes.io/instance"])
}

// (f) predicate: an Update with unchanged capacity+labels is filtered out, while
// an Update that adds a relevant capacity key passes.
func TestNodeLabeler_Predicate(t *testing.T) {
	pred := nodeCapacityOrLabelPredicate()

	base := node("n", map[string]string{GPUPresentLabel: "true"}, corev1.ResourceList{
		nvidiaGPUResource: resource.MustParse("1"),
	})

	// Unchanged relevant fields (only an irrelevant condition/heartbeat differs).
	unchangedNew := base.DeepCopy()
	unchangedNew.Status.Allocatable = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: unchangedNew}))

	// New relevant capacity key appears.
	changedNew := base.DeepCopy()
	changedNew.Status.Capacity[neuronResource] = resource.MustParse("1")
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: changedNew}))

	// A managed label change also passes.
	labelChangedNew := base.DeepCopy()
	labelChangedNew.Labels = map[string]string{}
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: labelChangedNew}))

	// Create always reconciles; Delete/Generic never do.
	assert.True(t, pred.Create(event.CreateEvent{Object: base}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: base}))
	assert.False(t, pred.Generic(event.GenericEvent{Object: base}))
}
