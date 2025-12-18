// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/jmx"
)

func Test_getDefaultInstrumentationLinux(t *testing.T) {
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA", defaultJavaInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON", defaultPythonInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET", defaultDotNetInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS", defaultNodeJSInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_LIMIT", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_REQUEST", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_LIMIT", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_REQUEST", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_REQUEST", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_REQUEST", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_RUNTIME_ENABLED", "true")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_RUNTIME_ENABLED", "true")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_RUNTIME_ENABLED", "true")

	httpInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}
	httpsInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=https://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}

	type args struct {
		agentConfig *adapters.CwaConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.Instrumentation
		wantErr bool
	}{
		{
			name: "application-signals-http",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{
							ApplicationSignals: &adapters.AppSignals{},
						},
					},
				},
			},
			want:    httpInst,
			wantErr: false,
		},
		{
			name: "application-signals-https",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{
							ApplicationSignals: &adapters.AppSignals{
								TLS: &adapters.TLS{
									CertFile: "some-cert",
									KeyFile:  "some-key",
								},
							},
						},
					},
				},
			},
			want:    httpsInst,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDefaultInstrumentation(tt.args.agentConfig, nil, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDefaultInstrumentation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultInstrumentation() got = %v, want %v", got, tt.want)
			}
		})
	}

}

func Test_getDefaultInstrumentationWindows(t *testing.T) {
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA", defaultJavaInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON", defaultPythonInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET", defaultDotNetInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS", defaultNodeJSInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_LIMIT", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_REQUEST", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_LIMIT", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_REQUEST", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_REQUEST", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_REQUEST", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_RUNTIME_METRICS", "true")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_RUNTIME_METRICS", "true")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_RUNTIME_METRICS", "true")

	httpInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}
	httpsInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}

	type args struct {
		agentConfig *adapters.CwaConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.Instrumentation
		wantErr bool
	}{
		{
			name: "application-signals-http",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{
							ApplicationSignals: &adapters.AppSignals{},
						},
					},
				},
			},
			want:    httpInst,
			wantErr: false,
		},
		{
			name: "application-signals-https",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{
							ApplicationSignals: &adapters.AppSignals{
								TLS: &adapters.TLS{
									CertFile: "some-cert",
									KeyFile:  "some-key",
								},
							},
						},
					},
				},
			},
			want:    httpsInst,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDefaultInstrumentation(tt.args.agentConfig, nil, true)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDefaultInstrumentation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultInstrumentation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDefaultInstrumentationLinuxWithApplicationSignalsDisabled(t *testing.T) {
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA", defaultJavaInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON", defaultPythonInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET", defaultDotNetInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS", defaultNodeJSInstrumentationImage)
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_LIMIT", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_JAVA_MEM_REQUEST", "64Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_LIMIT", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_PYTHON_MEM_REQUEST", "32Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_DOTNET_MEM_REQUEST", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_LIMIT", "500m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_LIMIT", "128Mi")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_CPU_REQUEST", "50m")
	_ = os.Setenv("AUTO_INSTRUMENTATION_NODEJS_MEM_REQUEST", "128Mi")

	httpInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_TRACES_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}

	jmxInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumentation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_TRACES_EXPORTER", Value: "none"},
					{Name: "OTEL_AWS_JMX_EXPORTER_METRICS_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4314/v1/metrics"},
					{Name: "OTEL_JMX_TARGET_SYSTEM", Value: "jvm"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			Python: v1alpha1.Python{
				Image: defaultPythonInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: defaultNodeJSInstrumentationImage,
				Env:   []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}

	type args struct {
		agentConfig    *adapters.CwaConfig
		additionalEnvs map[Type]map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.Instrumentation
		wantErr bool
	}{
		{
			name: "no-application-signals",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{},
					},
				},
			},
			want:    httpInst,
			wantErr: false,
		},
		{
			name: "no-application-signals-with-additional-vars",
			args: args{
				agentConfig: &adapters.CwaConfig{
					Logs: &adapters.Logs{
						LogMetricsCollected: &adapters.LogMetricsCollected{},
					},
				},
				additionalEnvs: map[Type]map[string]string{
					TypeJava: {
						jmx.EnvTargetSystem: jmx.TargetJVM,
					},
				},
			},
			want:    jmxInst,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDefaultInstrumentation(tt.args.agentConfig, tt.args.additionalEnvs, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDefaultInstrumentation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Spec.Java.Env, tt.want.Spec.Java.Env) {
				t.Errorf("getDefaultInstrumentation() Java environment vars got = %v, want %v", got, tt.want)
			}
		})
	}
}
