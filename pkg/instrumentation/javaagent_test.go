// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestInjectJavaagent(t *testing.T) {
	tests := []struct {
		name string
		v1alpha1.Java
		pod      corev1.Pod
		expected corev1.Pod
		err      error
	}{
		{
			name: "JAVA_TOOL_OPTIONS not defined",
			Java: v1alpha1.Java{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
			},
			expected: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "opentelemetry-auto-instrumentation-java",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    "opentelemetry-auto-instrumentation-java",
							Image:   "foo/bar:1",
							Command: []string{"cp", "/javaagent.jar", "/otel-auto-instrumentation-java/javaagent.jar"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-java",
								MountPath: "/otel-auto-instrumentation-java",
							}},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "opentelemetry-auto-instrumentation-java",
									MountPath: "/otel-auto-instrumentation-java",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "JAVA_TOOL_OPTIONS",
									Value: javaJVMArgument,
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "JAVA_TOOL_OPTIONS defined",
			Java: v1alpha1.Java{Image: "foo/bar:1", Resources: testResourceRequirements},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "JAVA_TOOL_OPTIONS",
									Value: "-Dbaz=bar",
								},
							},
						},
					},
				},
			},
			expected: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "opentelemetry-auto-instrumentation-java",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    "opentelemetry-auto-instrumentation-java",
							Image:   "foo/bar:1",
							Command: []string{"cp", "/javaagent.jar", "/otel-auto-instrumentation-java/javaagent.jar"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-java",
								MountPath: "/otel-auto-instrumentation-java",
							}},
							Resources: testResourceRequirements,
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "opentelemetry-auto-instrumentation-java",
									MountPath: "/otel-auto-instrumentation-java",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "JAVA_TOOL_OPTIONS",
									Value: "-Dbaz=bar" + javaJVMArgument,
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "JAVA_TOOL_OPTIONS defined as ValueFrom",
			Java: v1alpha1.Java{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:      "JAVA_TOOL_OPTIONS",
									ValueFrom: &corev1.EnvVarSource{},
								},
							},
						},
					},
				},
			},
			expected: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:      "JAVA_TOOL_OPTIONS",
									ValueFrom: &corev1.EnvVarSource{},
								},
							},
						},
					},
				},
			},
			err: fmt.Errorf("the container defines env var value via ValueFrom, envVar: %s", envJavaToolsOptions),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod, err := injectJavaagent(test.Java, test.pod, 0)
			assert.Equal(t, test.expected, pod)
			assert.Equal(t, test.err, err)
		})
	}
}
