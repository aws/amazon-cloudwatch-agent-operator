// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

const (
	testImagePython  = "public.ecr.aws/aws-observability/adot-autoinstrumentation-python:v0.0.1"
	testAppContainer = "app"
)

var testNow = time.Date(2026, time.September, 24, 18, 0, 0, 0, time.UTC)

// stampedPod builds a pod the stamp mutator would have produced for the given languages, as the
// controller sees it: the label the mutator sets, plus the init container statuses the kubelet
// reports for the init containers injection added.
func stampedPod(name string, images map[instrumentation.Type]string) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Labels:      map[string]string{instrumentation.LabelAutoInstrumented: "true"},
			Annotations: map[string]string{},
		},
	}
	for _, instType := range []instrumentation.Type{
		instrumentation.TypeJava,
		instrumentation.TypeNodeJS,
		instrumentation.TypePython,
		instrumentation.TypeDotNet,
	} {
		image, injected := images[instType]
		if !injected {
			continue
		}
		initContainer, ok := instrumentation.InitContainerName(instType)
		if !ok {
			continue
		}
		pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
			Name:  initContainer,
			Image: image,
		})
	}
	return pod
}

func withStartTime(pod corev1.Pod, startTime time.Time) corev1.Pod {
	t := metav1.NewTime(startTime)
	pod.Status.StartTime = &t
	return pod
}

func TestEvaluatePod(t *testing.T) {
	cfg := DefaultConfig()
	pythonInit, ok := instrumentation.InitContainerName(instrumentation.TypePython)
	require.True(t, ok)

	for _, tt := range []struct {
		name       string
		pod        corev1.Pod
		wantBroken bool
		wantReason string
	}{
		{
			name: "restarts below threshold",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("below", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: testAppContainer, RestartCount: 2}}
				return pod
			}(),
		},
		{
			name: "restarts at threshold",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("at", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name:         testAppContainer,
					RestartCount: 3,
					State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "container app restarted 3 times (CrashLoopBackOff)",
		},
		{
			name: "oom in current state",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("oom-state", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name:  testAppContainer,
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reasonOOMKilled, ExitCode: 137}},
				}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "container app was OOMKilled",
		},
		{
			name: "oom in last state",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("oom-last", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name:                 testAppContainer,
					RestartCount:         1,
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reasonOOMKilled, ExitCode: 137}},
				}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "container app was OOMKilled",
		},
		{
			name: "auto-instrumentation init container failed",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("init-failed", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
					Name:  pythonInit,
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
				}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "auto-instrumentation init container " + pythonInit + " (python) exited with code 1",
		},
		{
			name: "auto-instrumentation init container restarting",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("init-restarting", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: pythonInit, RestartCount: 4}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "auto-instrumentation init container " + pythonInit + " (python) restarted 4 times",
		},
		{
			name: "auto-instrumentation init container cannot pull its image, inside the patience window",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("pull-waiting", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
					Name:  pythonInit,
					Image: testImagePython,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonImagePullBackOff}},
				}}
				return pod
			}(),
		},
		{
			name: "auto-instrumentation init container cannot pull its image, past the patience window",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("pull-stuck", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-6*time.Minute))
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
					Name:  pythonInit,
					Image: testImagePython,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonErrImagePull}},
				}}
				return pod
			}(),
			wantBroken: true,
			wantReason: "auto-instrumentation init container " + pythonInit + " (python) cannot pull image " + testImagePython + " (ErrImagePull)",
		},
		{
			name: "customer init container cannot pull its image",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("customer-pull", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-9*time.Minute))
				pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
					Name:  "migrate-db",
					Image: "registry.example.com/migrate:1",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonImagePullBackOff}},
				})
				return pod
			}(),
		},
		{
			name: "customer init container failed",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("customer-init", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
					Name:         "migrate-db",
					RestartCount: 9,
					State:        corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
				}}
				return pod
			}(),
		},
		{
			name: "start time outside the window",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("old", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Hour))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: testAppContainer, RestartCount: 12}}
				return pod
			}(),
		},
		{
			name: "no start time",
			pod: func() corev1.Pod {
				pod := stampedPod("unscheduled", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: testAppContainer, RestartCount: 12}}
				return pod
			}(),
		},
		{
			name: "crash looping agent sidecar",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("sidecar", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{Name: testAppContainer, RestartCount: 0},
					{
						Name:                 naming.Container(),
						RestartCount:         7,
						State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
						LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reasonOOMKilled}},
					},
				}
				return pod
			}(),
		},
		{
			name: "healthy pod",
			pod: func() corev1.Pod {
				pod := withStartTime(stampedPod("healthy", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), testNow.Add(-time.Minute))
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: testAppContainer, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: pythonInit, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
				return pod
			}(),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			verdict := EvaluatePod(tt.pod, testNow, cfg)
			assert.Equal(t, tt.wantBroken, verdict.Broken)
			if tt.wantBroken {
				assert.Equal(t, tt.wantReason, verdict.Reason)
			} else {
				assert.Empty(t, verdict.Reason)
			}
		})
	}
}

func TestInWindow(t *testing.T) {
	window := 10 * time.Minute
	pod := stampedPod("p", nil)
	assert.False(t, InWindow(pod, testNow, window), "a pod with no start time is never in the window")
	assert.True(t, InWindow(withStartTime(pod, testNow.Add(-time.Minute)), testNow, window))
	assert.True(t, InWindow(withStartTime(pod, testNow.Add(-window)), testNow, window), "exactly at the window boundary is still in the window")
	assert.False(t, InWindow(withStartTime(pod, testNow.Add(-window-time.Second)), testNow, window))
}
