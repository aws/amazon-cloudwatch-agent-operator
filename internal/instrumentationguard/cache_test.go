// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// fullPod is a pod with every section the transform has an opinion about populated: the metadata
// the guard reads, the status it reads, and a spec it must throw away.
//
// It is deliberately TERMINATING, so that the transform is exercised on a pod whose
// DeletionTimestamp is set. That is the field finding 2 added, and a nil one would assert nothing.
// A caller that needs a live pod should use fullPod(t) and clear DeletionTimestamp, as
// TestTrimPodForCacheKeepsALivePodUsable does; do not make this helper return a live pod, because
// the terminating case is the one the transform regressed on.
func fullPod(t *testing.T) *corev1.Pod {
	t.Helper()
	pythonInit, ok := instrumentation.InitContainerName(instrumentation.TypePython)
	require.True(t, ok)
	startTime := metav1.NewTime(testNow.Add(-2 * time.Minute))

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders-7d9f-abc",
			Namespace:         testNamespace,
			UID:               types.UID("5f1b6f3a-0c2e-4a1f-9a6b-1f0f6a8d2c31"),
			ResourceVersion:   "992371",
			Generation:        4,
			CreationTimestamp: metav1.NewTime(testNow.Add(-3 * time.Minute)),
			// A terminating pod: AttributeFailure reads this field to refuse to blame
			// auto-instrumentation for a pod the cluster is tearing down, so the transform has to
			// carry it through. EvaluatePod does not read it, which is why the broken verdict
			// below still holds.
			DeletionTimestamp: func() *metav1.Time { t := metav1.NewTime(testNow); return &t }(),
			Labels: map[string]string{
				instrumentation.LabelAutoInstrumented: "true",
				"app":                                 "orders",
			},
			Annotations: map[string]string{
				instrumentation.InjectAnnotationKey(instrumentation.TypePython): "true",
				"kubectl.kubernetes.io/restartedAt":                             "2026-09-24T18:00:00Z",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "ReplicaSet",
				Name:       "orders-7d9f",
				Controller: func() *bool { b := true; return &b }(),
			}},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet", Operation: metav1.ManagedFieldsOperationUpdate}},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "orders",
			NodeName:           "ip-10-0-1-42.ec2.internal",
			InitContainers: []corev1.Container{{
				Name:    pythonInit,
				Image:   testImagePython,
				Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation-python"},
			}},
			Containers: []corev1.Container{{
				Name:  testAppContainer,
				Image: "orders:1.4.2",
				Env:   []corev1.EnvVar{{Name: "PYTHONPATH", Value: "/otel-auto-instrumentation-python"}},
			}},
			Volumes: []corev1.Volume{{Name: "opentelemetry-auto-instrumentation-python"}},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			StartTime: &startTime,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name:  pythonInit,
				Image: testImagePython,
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         testAppContainer,
				Image:        "orders:1.4.2",
				RestartCount: 3,
				State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Message:  "ModuleNotFoundError: No module named 'brotli'",
				}},
			}},
		},
	}
}

func TestTrimPodForCache(t *testing.T) {
	original := fullPod(t)

	transformed, err := TrimPodForCache(original.DeepCopy())
	require.NoError(t, err)

	trimmed, ok := transformed.(*corev1.Pod)
	require.True(t, ok, "a pod must come back as a pod")

	t.Run("the kept metadata survives", func(t *testing.T) {
		assert.Equal(t, original.Name, trimmed.Name)
		assert.Equal(t, original.Namespace, trimmed.Namespace)
		assert.Equal(t, original.UID, trimmed.UID)
		assert.Equal(t, original.ResourceVersion, trimmed.ResourceVersion,
			"the cache tracks objects by resourceVersion")
		assert.Equal(t, original.CreationTimestamp, trimmed.CreationTimestamp)
		assert.Equal(t, original.DeletionTimestamp, trimmed.DeletionTimestamp,
			"AttributeFailure never blames us for a terminating pod, so it must be able to see one")
		assert.Equal(t, original.Labels, trimmed.Labels)
		assert.Equal(t, original.Annotations, trimmed.Annotations)
		assert.Equal(t, original.OwnerReferences, trimmed.OwnerReferences,
			"ResolveWorkload walks the owner references")
	})

	t.Run("the kept status survives", func(t *testing.T) {
		assert.Equal(t, original.Status.StartTime, trimmed.Status.StartTime)
		assert.Equal(t, original.Status.ContainerStatuses, trimmed.Status.ContainerStatuses)
		assert.Equal(t, original.Status.InitContainerStatuses, trimmed.Status.InitContainerStatuses)
	})

	t.Run("the spec is dropped entirely", func(t *testing.T) {
		assert.Equal(t, corev1.PodSpec{}, trimmed.Spec,
			"the spec is the bulk of a cached pod and the guard reads none of it")
	})

	t.Run("the unread status and metadata go too", func(t *testing.T) {
		assert.Empty(t, trimmed.Status.Phase)
		assert.Empty(t, trimmed.Status.Conditions, "IsReady is gone, so conditions are dead weight")
		assert.Empty(t, trimmed.ManagedFields)
		assert.Zero(t, trimmed.Generation)
	})

	t.Run("the guard still reads what it needs off the trimmed pod", func(t *testing.T) {
		assert.Equal(t, map[instrumentation.Type]string{instrumentation.TypePython: testImagePython},
			instrumentation.InjectedImages(*trimmed))
		assert.True(t, EvaluatePod(*trimmed, testNow, DefaultConfig()).Broken)
	})

	t.Run("the input is not mutated", func(t *testing.T) {
		assert.Equal(t, fullPod(t), original)
	})
}

// TestTrimPodForCacheFeedsAttribution covers what the "guard still reads what it needs" subtest
// above cannot. fullPod is mid-delete, and AttributeFailure refuses to blame auto-instrumentation
// for a terminating pod before it reads any status, so asserting attribution against it only
// re-tests the DeletionTimestamp guard.
//
// Both pods below are correctly NOT ours - fullPod's init container exits 0 and its application
// crashes with a ModuleNotFoundError naming nothing of ours. The point is that they say so for
// DIFFERENT reasons: the terminating pod short-circuits, while the live one is actually evaluated
// against the status the transform preserved. That is what proves the transform kept enough.
func TestTrimPodForCacheFeedsAttribution(t *testing.T) {
	trim := func(t *testing.T, pod *corev1.Pod) corev1.Pod {
		t.Helper()
		transformed, err := TrimPodForCache(pod.DeepCopy())
		require.NoError(t, err)
		trimmed, ok := transformed.(*corev1.Pod)
		require.True(t, ok)
		return *trimmed
	}

	t.Run("a terminating pod short-circuits before any status is read", func(t *testing.T) {
		terminating := trim(t, fullPod(t))
		require.NotNil(t, terminating.DeletionTimestamp)

		attribution := AttributeFailure(terminating, nil)
		assert.False(t, attribution.Ours)
		assert.Contains(t, attribution.Reason, "terminating")
	})

	t.Run("a live pod is evaluated against the preserved status", func(t *testing.T) {
		live := fullPod(t)
		live.DeletionTimestamp = nil
		trimmed := trim(t, live)
		require.Nil(t, trimmed.DeletionTimestamp, "a live pod must not come back looking terminating")

		attribution := AttributeFailure(trimmed, nil)
		assert.False(t, attribution.Ours, "this fixture's failure genuinely is not ours")
		assert.NotContains(t, attribution.Reason, "terminating",
			"the live pod must get past the DeletionTimestamp guard")
		assert.Contains(t, attribution.Reason, "no auto-instrumentation frame",
			"attribution must have reached the message, which means the transform kept it")
	})
}

func TestTrimPodForCacheLeavesANonPodAlone(t *testing.T) {
	// A DeletedFinalStateUnknown tombstone, or any other type: dropping it would lose the event,
	// so it has to come back untouched and without an error.
	for _, input := range []interface{}{
		newTestDeployment("orders"),
		"a tombstone, as far as this function is concerned",
		nil,
	} {
		got, err := TrimPodForCache(input)
		assert.NoError(t, err)
		assert.Equal(t, input, got)
	}
}
