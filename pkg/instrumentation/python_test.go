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

func TestInjectPythonSDK(t *testing.T) {
	tests := []struct {
		name string
		v1alpha1.Python
		pod      corev1.Pod
		expected corev1.Pod
		err      error
	}{
		{
			name:   "PYTHONPATH not defined",
			Python: v1alpha1.Python{Image: "foo/bar:1"},
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
							Name: certVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: pythonVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  initCertContainerName,
							Image: shellContainerName,
							Command: []string{"/bin/sh", "-c",
								"mkdir -p amazon-cloudwatch-agent &&  echo 'open /etc/amazon-cloudwatch-app-signals-cert/tls-ca.crt: no such file or directory'  > ./amazon-cloudwatch-agent/ca.crt"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      certVolumeName,
								MountPath: certVolumePath,
							}},
							WorkingDir: certVolumePath,
						},
						{
							Name:    "opentelemetry-auto-instrumentation-python",
							Image:   "foo/bar:1",
							Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation-python"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-python",
								MountPath: "/otel-auto-instrumentation-python",
							}},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certVolumeName,
									MountPath: certVolumePath,
								},
								{
									Name:      "opentelemetry-auto-instrumentation-python",
									MountPath: "/otel-auto-instrumentation-python",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PYTHONPATH",
									Value: fmt.Sprintf("%s:%s", "/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation", "/otel-auto-instrumentation-python"),
								},
								{
									Name:  "OTEL_TRACES_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
									Value: "http/protobuf",
								},
								{
									Name:  "OTEL_METRICS_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
									Value: "http/protobuf",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:   "PYTHONPATH defined",
			Python: v1alpha1.Python{Image: "foo/bar:1", Resources: testResourceRequirements},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "PYTHONPATH",
									Value: "/foo:/bar",
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
							Name: certVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: "opentelemetry-auto-instrumentation-python",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  initCertContainerName,
							Image: shellContainerName,
							Command: []string{"/bin/sh", "-c",
								"mkdir -p amazon-cloudwatch-agent &&  echo 'open /etc/amazon-cloudwatch-app-signals-cert/tls-ca.crt: no such file or directory'  > ./amazon-cloudwatch-agent/ca.crt"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      certVolumeName,
								MountPath: certVolumePath,
							}},
							Resources:  testResourceRequirements,
							WorkingDir: certVolumePath,
						},
						{
							Name:    "opentelemetry-auto-instrumentation-python",
							Image:   "foo/bar:1",
							Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation-python"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-python",
								MountPath: "/otel-auto-instrumentation-python",
							}},
							Resources: testResourceRequirements,
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certVolumeName,
									MountPath: certVolumePath,
								},
								{
									Name:      "opentelemetry-auto-instrumentation-python",
									MountPath: "/otel-auto-instrumentation-python",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PYTHONPATH",
									Value: fmt.Sprintf("%s:%s:%s", "/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation", "/foo:/bar", "/otel-auto-instrumentation-python"),
								},
								{
									Name:  "OTEL_TRACES_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
									Value: "http/protobuf",
								},
								{
									Name:  "OTEL_METRICS_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
									Value: "http/protobuf",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:   "OTEL_TRACES_EXPORTER defined",
			Python: v1alpha1.Python{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_TRACES_EXPORTER",
									Value: "zipkin",
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
							Name: certVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: pythonVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  initCertContainerName,
							Image: shellContainerName,
							Command: []string{"/bin/sh", "-c",
								"mkdir -p amazon-cloudwatch-agent &&  echo 'open /etc/amazon-cloudwatch-app-signals-cert/tls-ca.crt: no such file or directory'  > ./amazon-cloudwatch-agent/ca.crt"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      certVolumeName,
								MountPath: certVolumePath,
							}},
							WorkingDir: certVolumePath,
						},
						{
							Name:    "opentelemetry-auto-instrumentation-python",
							Image:   "foo/bar:1",
							Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation-python"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-python",
								MountPath: "/otel-auto-instrumentation-python",
							}},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certVolumeName,
									MountPath: certVolumePath,
								},
								{
									Name:      "opentelemetry-auto-instrumentation-python",
									MountPath: "/otel-auto-instrumentation-python",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_TRACES_EXPORTER",
									Value: "zipkin",
								},
								{
									Name:  "PYTHONPATH",
									Value: fmt.Sprintf("%s:%s", "/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation", "/otel-auto-instrumentation-python"),
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
									Value: "http/protobuf",
								},
								{
									Name:  "OTEL_METRICS_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
									Value: "http/protobuf",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:   "OTEL_METRICS_EXPORTER defined",
			Python: v1alpha1.Python{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_METRICS_EXPORTER",
									Value: "somebackend",
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
							Name: certVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: "opentelemetry-auto-instrumentation-python",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  initCertContainerName,
							Image: shellContainerName,
							Command: []string{"/bin/sh", "-c",
								"mkdir -p amazon-cloudwatch-agent &&  echo 'open /etc/amazon-cloudwatch-app-signals-cert/tls-ca.crt: no such file or directory'  > ./amazon-cloudwatch-agent/ca.crt"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      certVolumeName,
								MountPath: certVolumePath,
							}},
							WorkingDir: certVolumePath,
						},
						{
							Name:    "opentelemetry-auto-instrumentation-python",
							Image:   "foo/bar:1",
							Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation-python"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "opentelemetry-auto-instrumentation-python",
								MountPath: "/otel-auto-instrumentation-python",
							}},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certVolumeName,
									MountPath: certVolumePath,
								},
								{
									Name:      "opentelemetry-auto-instrumentation-python",
									MountPath: "/otel-auto-instrumentation-python",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_METRICS_EXPORTER",
									Value: "somebackend",
								},
								{
									Name:  "PYTHONPATH",
									Value: fmt.Sprintf("%s:%s", "/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation", "/otel-auto-instrumentation-python"),
								},
								{
									Name:  "OTEL_TRACES_EXPORTER",
									Value: "otlp",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
									Value: "http/protobuf",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
									Value: "http/protobuf",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:   "PYTHONPATH defined as ValueFrom",
			Python: v1alpha1.Python{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:      "PYTHONPATH",
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
									Name:      "PYTHONPATH",
									ValueFrom: &corev1.EnvVarSource{},
								},
							},
						},
					},
				},
			},
			err: fmt.Errorf("the container defines env var value via ValueFrom, envVar: %s", envPythonPath),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod, err := injectPythonSDK(test.Python, test.pod, 0)
			assert.Equal(t, test.expected, pod)
			assert.Equal(t, test.err, err)
		})
	}
}
