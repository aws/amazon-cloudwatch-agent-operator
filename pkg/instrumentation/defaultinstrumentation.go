// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"errors"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	defaultAPIVersion     = "cloudwatch.aws.amazon.com/v1alpha1"
	defaultInstrumenation = "java-instrumentation"
	defaultNamespace      = "default"
	defaultKind           = "Instrumentation"

	otelSampleEnabledKey                       = "OTEL_SMP_ENABLED" //TODO: remove in favor of new name once safe
	otelSampleEnabledDefaultValue              = "true"             //TODO: remove in favor of new name once safe
	otelAppSignalsEnabledKey                   = "OTEL_AWS_APP_SIGNALS_ENABLED"
	otelAppSignalsEnabledDefaultValue          = "true"
	otelTracesSamplerArgKey                    = "OTEL_TRACES_SAMPLER_ARG"
	otelTracesSamplerArgDefaultValue           = "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"
	otelTracesSamplerKey                       = "OTEL_TRACES_SAMPLER"
	otelTracesSamplerDefaultValue              = "xray"
	otelExporterOtlpProtocolKey                = "OTEL_EXPORTER_OTLP_PROTOCOL"
	otelExporterOtlpProtocolValue              = "http/protobuf"
	otelExporterTracesEndpointKey              = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	otelExporterTracesEndpointDefaultValue     = "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"
	otelExporterSmpEndpointKey                 = "OTEL_AWS_SMP_EXPORTER_ENDPOINT"                 //TODO: remove in favor of new name once safe
	otelExporterSmpEndpointDefaultValue        = "http://cloudwatch-agent.amazon-cloudwatch:4315" //TODO: remove in favor of new name once safe
	otelExporterAppSignalsEndpointKey          = "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT"
	otelExporterAppSignalsEndpointDefaultValue = "http://cloudwatch-agent.amazon-cloudwatch:4315"
	otelExporterMetricKey                      = "OTEL_METRICS_EXPORTER"
	otelExporterMetricDefaultValue             = "none"
	otelExporterLogsKey                        = "OTEL_LOGS_EXPORTER"
	otelExporterLogsDefaultValue               = "none"

	otelPythonDistro                   = "OTEL_PYTHON_DISTRO"
	otelPythonDistroDefaultValue       = "aws_distro"
	otelPythonConfigurator             = "OTEL_PYTHON_CONFIGURATOR"
	otelPythonConfiguratorDefaultValue = "aws_configurator"
)

func getDefaultInstrumentation() (*v1alpha1.Instrumentation, error) {
	javaInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_JAVA")
	if !ok {
		return nil, errors.New("unable to determine java instrumentation image")
	}
	pythonInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_PYTHON")
	if !ok {
		return nil, errors.New("unable to determine python instrumentation image")
	}

	return &v1alpha1.Instrumentation{
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
				Image: javaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: otelSampleEnabledKey, Value: otelSampleEnabledDefaultValue}, //TODO: remove in favor of new name once safe
					{Name: otelAppSignalsEnabledKey, Value: otelAppSignalsEnabledDefaultValue},
					{Name: otelTracesSamplerArgKey, Value: otelTracesSamplerArgDefaultValue},
					{Name: otelTracesSamplerKey, Value: otelTracesSamplerDefaultValue},
					{Name: otelExporterOtlpProtocolKey, Value: otelExporterOtlpProtocolValue},
					{Name: otelExporterTracesEndpointKey, Value: otelExporterTracesEndpointDefaultValue},
					{Name: otelExporterSmpEndpointKey, Value: otelExporterSmpEndpointDefaultValue}, //TODO: remove in favor of new name once safe
					{Name: otelExporterAppSignalsEndpointKey, Value: otelExporterAppSignalsEndpointDefaultValue},
					{Name: otelExporterMetricKey, Value: otelExporterMetricDefaultValue},
					{Name: otelExporterLogsKey, Value: otelExporterLogsDefaultValue},
				},
			},
			Python: v1alpha1.Python{
				Image: pythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: otelAppSignalsEnabledKey, Value: otelAppSignalsEnabledDefaultValue},
					{Name: otelTracesSamplerArgKey, Value: otelTracesSamplerArgDefaultValue},
					{Name: otelTracesSamplerKey, Value: otelTracesSamplerDefaultValue},
					{Name: otelExporterOtlpProtocolKey, Value: otelExporterOtlpProtocolValue},
					{Name: otelExporterTracesEndpointKey, Value: otelExporterTracesEndpointDefaultValue},
					{Name: otelExporterAppSignalsEndpointKey, Value: otelExporterAppSignalsEndpointDefaultValue},
					{Name: otelExporterMetricKey, Value: otelExporterMetricDefaultValue},
					{Name: otelPythonDistro, Value: otelPythonDistroDefaultValue},
					{Name: otelPythonConfigurator, Value: otelPythonConfiguratorDefaultValue},
					{Name: otelExporterLogsKey, Value: otelExporterLogsDefaultValue},
				},
			},
		},
	}, nil
}
