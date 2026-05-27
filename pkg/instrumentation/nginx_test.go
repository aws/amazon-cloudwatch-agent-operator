// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestInjectNginxSDK(t *testing.T) {

	tests := []struct {
		name string
		v1alpha1.Nginx
		pod      corev1.Pod
		expected corev1.Pod
	}{
		{
			name: "Clone Container not present",
			Nginx: v1alpha1.Nginx{
				Image: "foo/bar:1",
				Attrs: []corev1.EnvVar{
					{
						Name:  "NginxModuleOtelMaxQueueSize",
						Value: "4096",
					},
				},
			},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
			},
			expected: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: nginxAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: nginxAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    nginxAgentCloneContainerName,
							Image:   "",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxCloneScript, "--", "/etc/nginx"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      nginxAgentConfigVolume,
								MountPath: nginxAgentConfDirFull,
							}},
						},
						{
							Name:    nginxAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxAgentScript, "--", "nginx.conf"},
							Env: []corev1.EnvVar{
								{
									Name:  nginxAttributesEnvVar,
									Value: "NginxModuleEnabled ON;\nNginxModuleOtelExporterEndpoint http://otlp-endpoint:4317;\nNginxModuleOtelMaxQueueSize 4096;\nNginxModuleOtelSpanExporter otlp;\nNginxModuleResolveBackends ON;\nNginxModuleServiceInstanceId <<SID-PLACEHOLDER>>;\nNginxModuleServiceName nginx-service-name;\nNginxModuleServiceNamespace req-namespace;\nNginxModuleTraceAsError ON;\n",
								},
								{
									Name: nginxServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: nginxAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: "/etc/nginx",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/opt/opentelemetry-webserver/agent/sdk_lib/lib",
								},
							},
						},
					},
				},
			},
		},
		// === Test ConfigFile configuration =============================
		{
			name: "ConfigFile honored",
			Nginx: v1alpha1.Nginx{
				Image:      "foo/bar:1",
				ConfigFile: "/opt/nginx/custom-nginx.conf",
			},
			pod: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: nginxAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: nginxAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    nginxAgentCloneContainerName,
							Image:   "",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxCloneScript, "--", "/opt/nginx"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      nginxAgentConfigVolume,
								MountPath: nginxAgentConfDirFull,
							}},
						},
						{
							Name:    nginxAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxAgentScript, "--", "custom-nginx.conf"},
							Env: []corev1.EnvVar{
								{
									Name:  nginxAttributesEnvVar,
									Value: "NginxModuleEnabled ON;\nNginxModuleOtelExporterEndpoint http://otlp-endpoint:4317;\nNginxModuleOtelSpanExporter otlp;\nNginxModuleResolveBackends ON;\nNginxModuleServiceInstanceId <<SID-PLACEHOLDER>>;\nNginxModuleServiceName nginx-service-name;\nNginxModuleServiceNamespace req-namespace;\nNginxModuleTraceAsError ON;\n",
								},
								{
									Name: nginxServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: nginxAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: "/opt/nginx",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/opt/opentelemetry-webserver/agent/sdk_lib/lib",
								},
							},
						},
					},
				},
			}},
		// === Test Removal of probes and lifecycle =============================
		{
			name: "Probes removed on clone init container",
			Nginx: v1alpha1.Nginx{
				Image: "foo/bar:1",
			},
			pod: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							ReadinessProbe: &corev1.Probe{},
							StartupProbe:   &corev1.Probe{},
							LivenessProbe:  &corev1.Probe{},
							Lifecycle:      &corev1.Lifecycle{},
						},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: nginxAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: nginxAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    nginxAgentCloneContainerName,
							Image:   "",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxCloneScript, "--", "/etc/nginx"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      nginxAgentConfigVolume,
								MountPath: nginxAgentConfDirFull,
							}},
						},
						{
							Name:    nginxAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxAgentScript, "--", "nginx.conf"},
							Env: []corev1.EnvVar{
								{
									Name:  nginxAttributesEnvVar,
									Value: "NginxModuleEnabled ON;\nNginxModuleOtelExporterEndpoint http://otlp-endpoint:4317;\nNginxModuleOtelSpanExporter otlp;\nNginxModuleResolveBackends ON;\nNginxModuleServiceInstanceId <<SID-PLACEHOLDER>>;\nNginxModuleServiceName nginx-service-name;\nNginxModuleServiceNamespace req-namespace;\nNginxModuleTraceAsError ON;\n",
								},
								{
									Name: nginxServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: nginxAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: "/etc/nginx",
								},
							},
							ReadinessProbe: &corev1.Probe{},
							StartupProbe:   &corev1.Probe{},
							LivenessProbe:  &corev1.Probe{},
							Lifecycle:      &corev1.Lifecycle{},
							Env: []corev1.EnvVar{
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/opt/opentelemetry-webserver/agent/sdk_lib/lib",
								},
							},
						},
					},
				},
			},
		},
		// Pod Namespace specified
		{
			name:  "Pod Namespace specified",
			Nginx: v1alpha1.Nginx{Image: "foo/bar:1"},
			pod: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "my-namespace",
					Name:      "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "my-namespace",
					Name:      "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: nginxAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: nginxAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    nginxAgentCloneContainerName,
							Image:   "",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxCloneScript, "--", "/etc/nginx"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      nginxAgentConfigVolume,
								MountPath: nginxAgentConfDirFull,
							}},
						},
						{
							Name:    nginxAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxAgentScript, "--", "nginx.conf"},
							Env: []corev1.EnvVar{
								{
									Name:  nginxAttributesEnvVar,
									Value: "NginxModuleEnabled ON;\nNginxModuleOtelExporterEndpoint http://otlp-endpoint:4317;\nNginxModuleOtelSpanExporter otlp;\nNginxModuleResolveBackends ON;\nNginxModuleServiceInstanceId <<SID-PLACEHOLDER>>;\nNginxModuleServiceName nginx-service-name;\nNginxModuleServiceNamespace my-namespace;\nNginxModuleTraceAsError ON;\n",
								},
								{
									Name: nginxServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: nginxAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: "/etc/nginx",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/opt/opentelemetry-webserver/agent/sdk_lib/lib",
								},
							},
						},
					},
				},
			},
		},
	}

	resourceMap := map[string]string{
		string(semconv.K8SDeploymentNameKey): "nginx-service-name",
		string(semconv.K8SNamespaceNameKey):  "req-namespace",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := injectNginxSDK(logr.Discard(), test.Nginx, test.pod, 0, "http://otlp-endpoint:4317", resourceMap)
			assert.Equal(t, test.expected, pod)
		})
	}
}

func TestInjectNginxUnknownNamespace(t *testing.T) {

	tests := []struct {
		name string
		v1alpha1.Nginx
		pod      corev1.Pod
		expected corev1.Pod
	}{
		{
			name:  "Clone Container not present, unknown namespace",
			Nginx: v1alpha1.Nginx{Image: "foo/bar:1"},
			pod: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "my-nginx-6c44bcbdd",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: nginxAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: nginxAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    nginxAgentCloneContainerName,
							Image:   "",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxCloneScript, "--", "/etc/nginx"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      nginxAgentConfigVolume,
								MountPath: nginxAgentConfDirFull,
							}},
						},
						{
							Name:    nginxAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{nginxAgentScript, "--", "nginx.conf"},
							Env: []corev1.EnvVar{
								{
									Name:  nginxAttributesEnvVar,
									Value: "NginxModuleEnabled ON;\nNginxModuleOtelExporterEndpoint http://otlp-endpoint:4317;\nNginxModuleOtelSpanExporter otlp;\nNginxModuleResolveBackends ON;\nNginxModuleServiceInstanceId <<SID-PLACEHOLDER>>;\nNginxModuleServiceName nginx-service-name;\nNginxModuleServiceNamespace nginx;\nNginxModuleTraceAsError ON;\n",
								},
								{
									Name: nginxServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: nginxAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      nginxAgentVolume,
									MountPath: nginxAgentDirFull,
								},
								{
									Name:      nginxAgentConfigVolume,
									MountPath: "/etc/nginx",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/opt/opentelemetry-webserver/agent/sdk_lib/lib",
								},
							},
						},
					},
				},
			},
		},
	}

	resourceMap := map[string]string{
		string(semconv.K8SDeploymentNameKey): "nginx-service-name",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := injectNginxSDK(logr.Discard(), test.Nginx, test.pod, 0, "http://otlp-endpoint:4317", resourceMap)
			assert.Equal(t, test.expected, pod)
		})
	}
}

func TestNginxInitContainerMissing(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		expected bool
	}{
		{
			name: "InitContainer_Already_Inject",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "istio-init",
						},
						{
							Name: nginxAgentInitContainerName,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "InitContainer_Absent_1",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "istio-init",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "InitContainer_Absent_2",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isNginxInitContainerMissing(test.pod, nginxAgentInitContainerName)
			assert.Equal(t, test.expected, result)
		})
	}
}


// TestNginx_ConfigFile_PositionalArg_NoSplice exercises the P431312609
// hardening: the user-controlled Nginx.ConfigFile value must reach BOTH the
// clone and attach init containers only as a positional argument ($1), never
// spliced into the shell-parsed script body.
func TestNginx_ConfigFile_PositionalArg_NoSplice(t *testing.T) {
	const malicious = "/etc/nginx/nginx.conf\nrm -rf /"

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{}},
		},
	}
	spec := v1alpha1.Nginx{
		Image:      "foo/bar:1",
		ConfigFile: malicious,
	}

	got := injectNginxSDK(logr.Discard(), spec, pod, 0, "http://otlp-endpoint:4317", map[string]string{})

	var attach, clone *corev1.Container
	for i := range got.Spec.InitContainers {
		c := &got.Spec.InitContainers[i]
		switch c.Name {
		case nginxAgentInitContainerName:
			attach = c
		case nginxAgentCloneContainerName:
			clone = c
		}
	}
	require.NotNil(t, clone, "clone init container %q missing", nginxAgentCloneContainerName)
	require.NotNil(t, attach, "attach init container %q missing", nginxAgentInitContainerName)

	// Both init containers derive their positional arg from the same user
	// input via getNginxConfDir / getNginxConfFile — assert against those
	// functions so the test stays in lock-step with production splitting logic.
	expectedDir := getNginxConfDir(malicious)
	expectedFile := getNginxConfFile(malicious)

	// Clone container: passes the parent directory as $1.
	assert.Equal(t, []string{"/bin/sh", "-c"}, clone.Command)
	require.Len(t, clone.Args, 3)
	assert.Equal(t, nginxCloneScript, clone.Args[0])
	assert.Equal(t, "--", clone.Args[1])
	assert.Equal(t, expectedDir, clone.Args[2])
	assert.False(t, strings.Contains(clone.Args[0], "rm -rf"),
		"clone script body must not splice malicious tail")

	// Attach container: passes the basename as $1.
	assert.Equal(t, []string{"/bin/sh", "-c"}, attach.Command)
	require.Len(t, attach.Args, 3)
	assert.Equal(t, nginxAgentScript, attach.Args[0])
	assert.Equal(t, "--", attach.Args[1])
	assert.Equal(t, expectedFile, attach.Args[2])
	assert.False(t, strings.Contains(attach.Args[0], "rm -rf"),
		"attach script body must not splice malicious tail")
}
