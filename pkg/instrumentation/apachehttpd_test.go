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

func TestInjectApacheHttpdagent(t *testing.T) {
	tests := []struct {
		name string
		v1alpha1.ApacheHttpd
		pod      corev1.Pod
		expected corev1.Pod
	}{
		{
			name:        "Clone Container not present",
			ApacheHttpd: v1alpha1.ApacheHttpd{Image: "foo/bar:1"},
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
							Name: apacheAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: apacheAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    apacheAgentCloneContainerName,
							Image:   "",
							Command: []string{"cp", "-r", "/usr/local/apache2/conf/.", apacheAgentDirectory + apacheAgentConfigDirectory},
							Args:    nil,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      apacheAgentConfigVolume,
								MountPath: apacheAgentDirectory + apacheAgentConfigDirectory,
							}},
						},
						{
							Name:    apacheAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{apacheHttpdAgentScript, "--", "/usr/local/apache2/conf"},
							Env: []corev1.EnvVar{
								{
									Name:  apacheAttributesEnvVar,
									Value: "\n#Load the Otel Webserver SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_common.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_resources.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_trace.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_otlp_recordable.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so\n#Load the Otel ApacheModule SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_webserver_sdk.so\n#Load the Apache Module. In this example for Apache 2.4\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Load the Apache Module. In this example for Apache 2.2\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel22.so\nLoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Attributes\nApacheModuleEnabled ON\nApacheModuleOtelExporterEndpoint http://otlp-endpoint:4317\nApacheModuleOtelSpanExporter otlp\nApacheModuleResolveBackends  ON\nApacheModuleServiceInstanceId <<SID-PLACEHOLDER>>\nApacheModuleServiceName apache-httpd-service-name\nApacheModuleServiceNamespace req-namespace\nApacheModuleTraceAsError  ON\n",
								},
								{Name: apacheServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheDefaultConfigDirectory,
								},
							},
						},
					},
				},
			},
		},
		// === Test ConfigPath configuration =============================
		{
			name: "ConfigPath honored",
			ApacheHttpd: v1alpha1.ApacheHttpd{
				Image:      "foo/bar:1",
				ConfigPath: "/opt/customPath",
			},
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
							Name: apacheAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: apacheAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    apacheAgentCloneContainerName,
							Image:   "",
							Command: []string{"cp", "-r", "/opt/customPath/.", apacheAgentDirectory + apacheAgentConfigDirectory},
							Args:    nil,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      apacheAgentConfigVolume,
								MountPath: apacheAgentDirectory + apacheAgentConfigDirectory,
							}},
						},
						{
							Name:    apacheAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{apacheHttpdAgentScript, "--", "/opt/customPath"},
							Env: []corev1.EnvVar{
								{
									Name:  apacheAttributesEnvVar,
									Value: "\n#Load the Otel Webserver SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_common.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_resources.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_trace.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_otlp_recordable.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so\n#Load the Otel ApacheModule SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_webserver_sdk.so\n#Load the Apache Module. In this example for Apache 2.4\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Load the Apache Module. In this example for Apache 2.2\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel22.so\nLoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Attributes\nApacheModuleEnabled ON\nApacheModuleOtelExporterEndpoint http://otlp-endpoint:4317\nApacheModuleOtelSpanExporter otlp\nApacheModuleResolveBackends  ON\nApacheModuleServiceInstanceId <<SID-PLACEHOLDER>>\nApacheModuleServiceName apache-httpd-service-name\nApacheModuleServiceNamespace req-namespace\nApacheModuleTraceAsError  ON\n",
								},
								{Name: apacheServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: "/opt/customPath",
								},
							},
						},
					},
				},
			},
		},
		// === Test Removal of probes  =============================
		{
			name:        "Probes removed on clone init container",
			ApacheHttpd: v1alpha1.ApacheHttpd{Image: "foo/bar:1"},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							ReadinessProbe: &corev1.Probe{},
							StartupProbe:   &corev1.Probe{},
							LivenessProbe:  &corev1.Probe{},
						},
					},
				},
			},
			expected: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: apacheAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: apacheAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    apacheAgentCloneContainerName,
							Image:   "",
							Command: []string{"cp", "-r", "/usr/local/apache2/conf/.", apacheAgentDirectory + apacheAgentConfigDirectory},
							Args:    nil,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      apacheAgentConfigVolume,
								MountPath: apacheAgentDirectory + apacheAgentConfigDirectory,
							}},
						},
						{
							Name:    apacheAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{apacheHttpdAgentScript, "--", "/usr/local/apache2/conf"},
							Env: []corev1.EnvVar{
								{
									Name:  apacheAttributesEnvVar,
									Value: "\n#Load the Otel Webserver SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_common.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_resources.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_trace.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_otlp_recordable.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so\n#Load the Otel ApacheModule SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_webserver_sdk.so\n#Load the Apache Module. In this example for Apache 2.4\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Load the Apache Module. In this example for Apache 2.2\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel22.so\nLoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Attributes\nApacheModuleEnabled ON\nApacheModuleOtelExporterEndpoint http://otlp-endpoint:4317\nApacheModuleOtelSpanExporter otlp\nApacheModuleResolveBackends  ON\nApacheModuleServiceInstanceId <<SID-PLACEHOLDER>>\nApacheModuleServiceName apache-httpd-service-name\nApacheModuleServiceNamespace req-namespace\nApacheModuleTraceAsError  ON\n",
								},
								{Name: apacheServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheDefaultConfigDirectory,
								},
							},
							ReadinessProbe: &corev1.Probe{},
							StartupProbe:   &corev1.Probe{},
							LivenessProbe:  &corev1.Probe{},
						},
					},
				},
			},
		},
		// Pod Namespace specified
		{
			name:        "Pod Namespace specified",
			ApacheHttpd: v1alpha1.ApacheHttpd{Image: "foo/bar:1"},
			pod: corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "my-namespace",
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
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: apacheAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: apacheAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    apacheAgentCloneContainerName,
							Image:   "",
							Command: []string{"cp", "-r", "/usr/local/apache2/conf/.", apacheAgentDirectory + apacheAgentConfigDirectory},
							Args:    nil,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      apacheAgentConfigVolume,
								MountPath: apacheAgentDirectory + apacheAgentConfigDirectory,
							}},
						},
						{
							Name:    apacheAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{apacheHttpdAgentScript, "--", "/usr/local/apache2/conf"},
							Env: []corev1.EnvVar{
								{
									Name:  apacheAttributesEnvVar,
									Value: "\n#Load the Otel Webserver SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_common.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_resources.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_trace.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_otlp_recordable.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so\n#Load the Otel ApacheModule SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_webserver_sdk.so\n#Load the Apache Module. In this example for Apache 2.4\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Load the Apache Module. In this example for Apache 2.2\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel22.so\nLoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Attributes\nApacheModuleEnabled ON\nApacheModuleOtelExporterEndpoint http://otlp-endpoint:4317\nApacheModuleOtelSpanExporter otlp\nApacheModuleResolveBackends  ON\nApacheModuleServiceInstanceId <<SID-PLACEHOLDER>>\nApacheModuleServiceName apache-httpd-service-name\nApacheModuleServiceNamespace my-namespace\nApacheModuleTraceAsError  ON\n",
								},
								{Name: apacheServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheDefaultConfigDirectory,
								},
							},
						},
					},
				},
			},
		},
	}

	resourceMap := map[string]string{
		string(semconv.K8SDeploymentNameKey): "apache-httpd-service-name",
		string(semconv.K8SNamespaceNameKey):  "req-namespace",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := injectApacheHttpdagent(logr.Discard(), test.ApacheHttpd, test.pod, 0, "http://otlp-endpoint:4317", resourceMap)
			assert.Equal(t, test.expected, pod)
		})
	}
}

func TestInjectApacheHttpdagentUnknownNamespace(t *testing.T) {
	tests := []struct {
		name string
		v1alpha1.ApacheHttpd
		pod      corev1.Pod
		expected corev1.Pod
	}{
		{
			name:        "Clone Container not present, unknown namespace",
			ApacheHttpd: v1alpha1.ApacheHttpd{Image: "foo/bar:1"},
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
							Name: apacheAgentConfigVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
						{
							Name: apacheAgentVolume,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: &defaultVolumeLimitSize,
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    apacheAgentCloneContainerName,
							Image:   "",
							Command: []string{"cp", "-r", "/usr/local/apache2/conf/.", apacheAgentDirectory + apacheAgentConfigDirectory},
							Args:    nil,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      apacheAgentConfigVolume,
								MountPath: apacheAgentDirectory + apacheAgentConfigDirectory,
							}},
						},
						{
							Name:    apacheAgentInitContainerName,
							Image:   "foo/bar:1",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{apacheHttpdAgentScript, "--", "/usr/local/apache2/conf"},
							Env: []corev1.EnvVar{
								{
									Name:  apacheAttributesEnvVar,
									Value: "\n#Load the Otel Webserver SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_common.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_resources.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_trace.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_otlp_recordable.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so\n#Load the Otel ApacheModule SDK\nLoadFile /opt/opentelemetry-webserver/agent/sdk_lib/lib/libopentelemetry_webserver_sdk.so\n#Load the Apache Module. In this example for Apache 2.4\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Load the Apache Module. In this example for Apache 2.2\n#LoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel22.so\nLoadModule otel_apache_module /opt/opentelemetry-webserver/agent/WebServerModule/Apache/libmod_apache_otel.so\n#Attributes\nApacheModuleEnabled ON\nApacheModuleOtelExporterEndpoint http://otlp-endpoint:4317\nApacheModuleOtelSpanExporter otlp\nApacheModuleResolveBackends  ON\nApacheModuleServiceInstanceId <<SID-PLACEHOLDER>>\nApacheModuleServiceName apache-httpd-service-name\nApacheModuleServiceNamespace apache-httpd\nApacheModuleTraceAsError  ON\n",
								},
								{Name: apacheServiceInstanceIdEnvVar,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheAgentConfDirFull,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      apacheAgentVolume,
									MountPath: apacheAgentDirectory + apacheAgentSubDirectory,
								},
								{
									Name:      apacheAgentConfigVolume,
									MountPath: apacheDefaultConfigDirectory,
								},
							},
						},
					},
				},
			},
		},
	}

	resourceMap := map[string]string{
		string(semconv.K8SDeploymentNameKey): "apache-httpd-service-name",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := injectApacheHttpdagent(logr.Discard(), test.ApacheHttpd, test.pod, 0, "http://otlp-endpoint:4317", resourceMap)
			assert.Equal(t, test.expected, pod)
		})
	}
}

func TestApacheInitContainerMissing(t *testing.T) {
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
							Name: apacheAgentInitContainerName,
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
			result := isApacheInitContainerMissing(test.pod, apacheAgentInitContainerName)
			assert.Equal(t, test.expected, result)
		})
	}
}


// TestApacheHttpd_ConfigPath_PositionalArg_NoSplice exercises the P431312609
// hardening: the user-controlled ApacheHttpd.ConfigPath value must reach the
// init container only as a positional argument ($1), never spliced into the
// shell-parsed script body.
func TestApacheHttpd_ConfigPath_PositionalArg_NoSplice(t *testing.T) {
	const malicious = "/tmp; touch /tmp/pwn #"

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{}},
		},
	}
	spec := v1alpha1.ApacheHttpd{
		Image:      "foo/bar:1",
		ConfigPath: malicious,
	}

	got := injectApacheHttpdagent(logr.Discard(), spec, pod, 0, "http://otlp-endpoint:4317", map[string]string{})

	var attach, clone *corev1.Container
	for i := range got.Spec.InitContainers {
		c := &got.Spec.InitContainers[i]
		switch c.Name {
		case apacheAgentInitContainerName:
			attach = c
		case apacheAgentCloneContainerName:
			clone = c
		}
	}
	require.NotNil(t, attach, "attach init container %q missing", apacheAgentInitContainerName)
	require.NotNil(t, clone, "clone init container %q missing", apacheAgentCloneContainerName)

	// Attach container must invoke the embedded script via positional arg.
	assert.Equal(t, []string{"/bin/sh", "-c"}, attach.Command)
	require.Len(t, attach.Args, 3)
	assert.Equal(t, apacheHttpdAgentScript, attach.Args[0])
	assert.Equal(t, "--", attach.Args[1])
	assert.Equal(t, malicious, attach.Args[2])

	// Script body (Args[0]) must NOT contain any user-supplied substring.
	assert.False(t, strings.Contains(attach.Args[0], "; touch"),
		"embedded script body must not splice user-supplied chars")
	assert.False(t, strings.Contains(attach.Args[0], "/tmp; touch"),
		"embedded script body must not splice user-supplied path")

	// Clone container must use exec-form cp (no shell), with Args=nil.
	expectedCloneCmd := []string{"cp", "-r", malicious + "/.", apacheAgentConfDirFull}
	assert.Equal(t, expectedCloneCmd, clone.Command)
	assert.Nil(t, clone.Args)
}
