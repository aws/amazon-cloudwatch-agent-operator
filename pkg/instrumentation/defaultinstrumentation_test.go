// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

func Test_getDefaultInstrumentationLinux(t *testing.T) {
	os.Setenv("AUTO_INSTRUMENTATION_JAVA", defaultJavaInstrumentationImage)
	os.Setenv("AUTO_INSTRUMENTATION_PYTHON", defaultPythonInstrumentationImage)
	os.Setenv("AUTO_INSTRUMENTATION_DOTNET", defaultDotNetInstrumentationImage)

	httpInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumenation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
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
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
			},
<<<<<<< HEAD
			// temporary environment variables. Need to be updated with the latest values
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
=======
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
>>>>>>> 5a9b93a4d2ead452d268ac760b95a52f0dec9546
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
<<<<<<< HEAD
=======
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
>>>>>>> 5a9b93a4d2ead452d268ac760b95a52f0dec9546
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
			Name:      defaultInstrumenation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
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
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
			},
<<<<<<< HEAD
			// temporary environment variables. Need to be updated with the latest values
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
=======
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
>>>>>>> 5a9b93a4d2ead452d268ac760b95a52f0dec9546
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
<<<<<<< HEAD
=======
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
>>>>>>> 5a9b93a4d2ead452d268ac760b95a52f0dec9546
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
			got, err := getDefaultInstrumentation(tt.args.agentConfig, false)
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
	os.Setenv("AUTO_INSTRUMENTATION_JAVA", defaultJavaInstrumentationImage)
	os.Setenv("AUTO_INSTRUMENTATION_PYTHON", defaultPythonInstrumentationImage)
	os.Setenv("AUTO_INSTRUMENTATION_DOTNET", defaultDotNetInstrumentationImage)

	httpInst := &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumenation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
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
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
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
			Name:      defaultInstrumenation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/traces"},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: "https://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
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
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: defaultDotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
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
			got, err := getDefaultInstrumentation(tt.args.agentConfig, true)
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
