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
	defaultAPIVersion                      = "cloudwatch.aws.amazon.com/v1alpha1"
	defaultInstrumenation                  = "java-instrumentation"
	defaultNamespace                       = "default"
	defaultKind                            = "Instrumentation"
	otelSampleEnabledKey                   = "OTEL_SMP_ENABLED"
	otelSampleEnabledDefaultValue          = "true"
	otelTracesSamplerArgKey                = "OTEL_TRACES_SAMPLER_ARG"
	otelTracesSamplerArgDefaultValue       = "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"
	otelTracesSamplerKey                   = "OTEL_TRACES_SAMPLER"
	otelTracesSamplerDefaultValue          = "xray"
	otelExporterTracesEndpointKey          = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	otelExporterTracesEndpointDefaultValue = "http://cloudwatch-agent.amazon-cloudwatch:4315"
	otelExporterSmpEndpointKey             = "OTEL_AWS_SMP_EXPORTER_ENDPOINT"
	otelExporterSmpEndpointDefaultValue    = "http://cloudwatch-agent.amazon-cloudwatch:4315"
	otelExporterMetricKey                  = "OTEL_METRICS_EXPORTER"
	otelExporterMetricDefaultValue         = "none"
)

func getDefaultInstrumentation() (*v1alpha1.Instrumentation, error) {
	instrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_JAVA")
	if !ok {
		return nil, errors.New("unable to determine instrumentation image")
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
				Image: instrumentationImage,
				Env: []corev1.EnvVar{
					{Name: otelSampleEnabledKey, Value: otelSampleEnabledDefaultValue},
					{Name: otelTracesSamplerArgKey, Value: otelTracesSamplerArgDefaultValue},
					{Name: otelTracesSamplerKey, Value: otelTracesSamplerDefaultValue},
					{Name: otelExporterTracesEndpointKey, Value: otelExporterTracesEndpointDefaultValue},
					{Name: otelExporterSmpEndpointKey, Value: otelExporterSmpEndpointDefaultValue},
					{Name: otelExporterMetricKey, Value: otelExporterMetricDefaultValue},
				},
			},
		},
	}, nil
}
