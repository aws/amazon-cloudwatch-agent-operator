// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"errors"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

const (
	defaultAPIVersion      = "cloudwatch.aws.amazon.com/v1alpha1"
	defaultInstrumentation = "java-instrumentation"
	defaultNamespace       = "default"
	defaultKind            = "Instrumentation"

	http  = "http"
	https = "https"

	java    = "JAVA"
	python  = "PYTHON"
	dotNet  = "DOTNET"
	limit   = "LIMIT"
	request = "REQUEST"
)

func getInstrumentationConfigForResource(langStr string, resourceStr string) corev1.ResourceList {
	instrumentationConfigCpu, _ := os.LookupEnv("AUTO_INSTRUMENTATION_" + langStr + "_CPU_" + resourceStr)
	instrumentationConfigMemory, _ := os.LookupEnv("AUTO_INSTRUMENTATION_" + langStr + "_MEM_" + resourceStr)

	instrumentationConfigForResource := corev1.ResourceList{}
	instrumentationConfigCpuQuantity, err := resource.ParseQuantity(instrumentationConfigCpu)
	if err == nil {
		instrumentationConfigForResource[corev1.ResourceCPU] = instrumentationConfigCpuQuantity
	}
	instrumentationConfigMemoryQuantity, err := resource.ParseQuantity(instrumentationConfigMemory)
	if err == nil {
		instrumentationConfigForResource[corev1.ResourceMemory] = instrumentationConfigMemoryQuantity
	}
	return instrumentationConfigForResource
}

func getDefaultInstrumentation(agentConfig *adapters.CwaConfig, isWindowsPod bool) (*v1alpha1.Instrumentation, error) {
	javaInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_JAVA")
	if !ok {
		return nil, errors.New("unable to determine java instrumentation image")
	}
	pythonInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_PYTHON")
	if !ok {
		return nil, errors.New("unable to determine python instrumentation image")
	}
	nodeJSInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_NODEJS")
	if !ok {
		return nil, errors.New("unable to determine nodejs instrumentation image")
	}
	dotNetInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_DOTNET")
	if !ok {
		return nil, errors.New("unable to determine dotnet instrumentation image")
	}

	cloudwatchAgentServiceEndpoint := "cloudwatch-agent.amazon-cloudwatch"
	if isWindowsPod {
		// Windows pods use the headless service endpoint due to limitations with the agent on host network mode
		// https://kubernetes.io/docs/concepts/services-networking/windows-networking/#limitations
		cloudwatchAgentServiceEndpoint = "cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local"
	}

	// set protocol by checking cloudwatch agent config for tls setting
	exporterPrefix := http
	if agentConfig != nil {
		appSignalsConfig := agentConfig.GetApplicationSignalsConfig()
		if appSignalsConfig != nil && appSignalsConfig.TLS != nil {
			exporterPrefix = https
		}
	}

	return &v1alpha1.Instrumentation{
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
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: javaInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", http, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(java, limit),
					Requests: getInstrumentationConfigForResource(java, request),
				},
			},
			Python: v1alpha1.Python{
				Image: pythonInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", http, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)}, //TODO: remove in favor of new name once safe
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(python, limit),
					Requests: getInstrumentationConfigForResource(python, request),
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: dotNetInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", http, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_DISTRO", Value: "aws_distro"},
					{Name: "OTEL_DOTNET_CONFIGURATOR", Value: "aws_configurator"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
					{Name: "OTEL_DOTNET_AUTO_PLUGINS", Value: "AWS.Distro.OpenTelemetry.AutoInstrumentation.Plugin, AWS.Distro.OpenTelemetry.AutoInstrumentation"},
				},
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(dotNet, limit),
					Requests: getInstrumentationConfigForResource(dotNet, request),
				},
			},
			//TODO: temporary environment variables. Update with the latest values from the ADOT SDK for NodeJS
			NodeJS: v1alpha1.NodeJS{
				Image: nodeJSInstrumentationImage,
				Env: []corev1.EnvVar{
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
					{Name: "OTEL_TRACES_SAMPLER_ARG", Value: "endpoint=http://cloudwatch-agent.amazon-cloudwatch:2000"},
					{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
					{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
					{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: exporterPrefix + "cloudwatch-agent.amazon-cloudwatch:4316/v1/traces"},
					{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: exporterPrefix + "cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"},
					{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
					{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
				},
			},
		},
	}, nil
}
