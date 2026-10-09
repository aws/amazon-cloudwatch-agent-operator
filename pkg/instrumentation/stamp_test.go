// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

const (
	testJavaImage   = "public.ecr.aws/aws-observability/adot-autoinstrumentation-java:v1.0.0"
	testPythonImage = "public.ecr.aws/aws-observability/adot-autoinstrumentation-python:v0.2.0"
)

func TestInitContainerName(t *testing.T) {
	tests := []struct {
		instType Type
		want     string
		wantOK   bool
	}{
		{TypeJava, javaInitContainerName, true},
		{TypeNodeJS, nodejsInitContainerName, true},
		{TypePython, pythonInitContainerName, true},
		{TypeDotNet, dotnetInitContainerName, true},
		{TypeGo, "", false},
	}
	for _, test := range tests {
		t.Run(string(test.instType), func(t *testing.T) {
			got, ok := InitContainerName(test.instType)
			assert.Equal(t, test.want, got)
			assert.Equal(t, test.wantOK, ok)
		})
	}
}

func TestInstrMountPath(t *testing.T) {
	tests := []struct {
		instType Type
		want     string
		wantOK   bool
	}{
		{TypeJava, "/otel-auto-instrumentation-java", true},
		{TypeNodeJS, "/otel-auto-instrumentation-nodejs", true},
		{TypePython, "/otel-auto-instrumentation-python", true},
		{TypeDotNet, "/otel-auto-instrumentation-dotnet", true},
		{TypeGo, "", false},
	}
	for _, test := range tests {
		t.Run(string(test.instType), func(t *testing.T) {
			got, ok := InstrMountPath(test.instType)
			assert.Equal(t, test.want, got)
			assert.Equal(t, test.wantOK, ok)
			volume, found := instrVolumeName(test.instType)
			assert.Equal(t, test.wantOK, found)
			if test.wantOK {
				assert.Equal(t, volumeName+"-"+string(test.instType), volume)
			} else {
				assert.Empty(t, volume)
			}
		})
	}
}

func TestStampMutator(t *testing.T) {
	tests := []struct {
		name       string
		pod        corev1.Pod
		wantLabels map[string]string
		// wantPolicies is the terminationMessagePolicy expected on each named container after
		// Mutate. Containers absent from the map must keep the policy they went in with.
		wantPolicies map[string]corev1.TerminationMessagePolicy
	}{
		{
			name: "no init containers, pod unchanged",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app"}},
				},
			},
			wantLabels: nil,
		},
		{
			name: "customer init container only, pod unchanged",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"a": "b"},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "wait-for-db", Image: "busybox"}},
					Containers:     []corev1.Container{{Name: "app"}},
				},
			},
			wantLabels: map[string]string{"app": "test"},
		},
		{
			name: "python injected, nil metadata maps",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: pythonInitContainerName, Image: testPythonImage}},
					Containers: []corev1.Container{{
						Name:                     "app",
						VolumeMounts:             []corev1.VolumeMount{{Name: pythonVolumeName, MountPath: pythonInstrMountPath}},
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					}},
				},
			},
			wantLabels: map[string]string{LabelAutoInstrumented: "true"},
			wantPolicies: map[string]corev1.TerminationMessagePolicy{
				"app": corev1.TerminationMessageFallbackToLogsOnError,
			},
		},
		{
			name: "only the injected container gets the policy",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{annotationInjectPython: "true"},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "wait-for-db", Image: "busybox"},
						{Name: javaInitContainerName, Image: testJavaImage},
						{Name: pythonInitContainerName, Image: testPythonImage},
					},
					Containers: []corev1.Container{
						{
							Name:                     "app",
							VolumeMounts:             []corev1.VolumeMount{{Name: pythonVolumeName, MountPath: pythonInstrMountPath}},
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							Name:                     "worker",
							VolumeMounts:             []corev1.VolumeMount{{Name: javaVolumeName, MountPath: javaInstrMountPath}},
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							// Not instrumented: no auto-instrumentation volume mount.
							Name:                     "nginx",
							VolumeMounts:             []corev1.VolumeMount{{Name: "config", MountPath: "/etc/nginx"}},
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							// The CloudWatch agent sidecar is never auto-instrumented.
							Name:                     naming.Container(),
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
				},
			},
			wantLabels: map[string]string{
				"app":                 "test",
				LabelAutoInstrumented: "true",
			},
			wantPolicies: map[string]corev1.TerminationMessagePolicy{
				"app":    corev1.TerminationMessageFallbackToLogsOnError,
				"worker": corev1.TerminationMessageFallbackToLogsOnError,
			},
		},
		{
			name: "go sidecar is not stamped",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app"}, {Name: sideCarName}},
				},
			},
			wantLabels: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Mutate takes the pod by value but shares the Containers backing array, so an
			// in-place mutation is visible through test.pod too. Snapshot the input first and
			// build the expected spec from that snapshot.
			before := test.pod.DeepCopy()
			wantSpec := before.Spec.DeepCopy()
			for i := range wantSpec.Containers {
				if policy, ok := test.wantPolicies[wantSpec.Containers[i].Name]; ok {
					wantSpec.Containers[i].TerminationMessagePolicy = policy
				}
			}

			got, err := NewStampMutator().Mutate(context.Background(), corev1.Namespace{}, test.pod)
			assert.NoError(t, err)
			assert.Equal(t, test.wantLabels, got.Labels)
			assert.Equal(t, before.Annotations, got.Annotations, "the mutator writes no annotations")
			assert.Equal(t, *wantSpec, got.Spec, "only terminationMessagePolicy may change")
		})
	}
}

func TestInjectedImages(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want map[Type]string
	}{
		{
			name: "not stamped",
			pod:  corev1.Pod{},
			want: map[Type]string{},
		},
		{
			name: "init container status without the label is ignored",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{{Name: pythonInitContainerName, Image: testPythonImage}},
			}},
			want: map[Type]string{},
		},
		{
			name: "customer init container is ignored",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelAutoInstrumented: "true"}},
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{{Name: "wait-for-db", Image: "busybox"}},
				},
			},
			want: map[Type]string{},
		},
		{
			name: "stamped with two languages",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelAutoInstrumented: "true"}},
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{
						{Name: "wait-for-db", Image: "busybox"},
						{Name: javaInitContainerName, Image: testJavaImage},
						{Name: pythonInitContainerName, Image: testPythonImage},
					},
				},
			},
			want: map[Type]string{TypeJava: testJavaImage, TypePython: testPythonImage},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, InjectedImages(test.pod))
		})
	}
}

// TestStampMutatorRoundTrip checks that what the mutator stamps in the webhook is what
// InjectedImages reads back in the controller, once the kubelet has reported a status.
func TestStampMutatorRoundTrip(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: javaInitContainerName, Image: testJavaImage},
				{Name: pythonInitContainerName, Image: testPythonImage},
			},
			Containers: []corev1.Container{{
				Name: "app",
				VolumeMounts: []corev1.VolumeMount{
					{Name: javaVolumeName, MountPath: javaInstrMountPath},
					{Name: pythonVolumeName, MountPath: pythonInstrMountPath},
				},
			}},
		},
	}
	stamped, err := NewStampMutator().Mutate(context.Background(), corev1.Namespace{}, pod)
	require.NoError(t, err)
	require.Equal(t, corev1.TerminationMessageFallbackToLogsOnError, stamped.Spec.Containers[0].TerminationMessagePolicy)

	// The kubelet reports one init container status per init container in the spec.
	for _, initContainer := range stamped.Spec.InitContainers {
		stamped.Status.InitContainerStatuses = append(stamped.Status.InitContainerStatuses, corev1.ContainerStatus{
			Name:  initContainer.Name,
			Image: initContainer.Image,
		})
	}

	assert.Equal(t, map[Type]string{TypeJava: testJavaImage, TypePython: testPythonImage}, InjectedImages(stamped))
}
