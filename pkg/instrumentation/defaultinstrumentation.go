// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"errors"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/jmx"
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
	nodeJS  = "NODEJS"
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

func getDefaultInstrumentation(agentConfig *adapters.CwaConfig, additionalEnvs map[Type]map[string]string, isWindowsPod bool) (*v1alpha1.Instrumentation, error) {
	javaInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_JAVA")
	if !ok {
		return nil, errors.New("unable to determine java instrumentation image")
	}
	pythonInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_PYTHON")
	if !ok {
		return nil, errors.New("unable to determine python instrumentation image")
	}
	dotNetInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_DOTNET")
	if !ok {
		return nil, errors.New("unable to determine dotnet instrumentation image")
	}
	nodeJSInstrumentationImage, ok := os.LookupEnv("AUTO_INSTRUMENTATION_NODEJS")
	if !ok {
		return nil, errors.New("unable to determine nodejs instrumentation image")
	}

	cloudwatchAgentServiceEndpoint := "cloudwatch-agent.amazon-cloudwatch"
	if isWindowsPod {
		// Windows pods use the headless service endpoint due to limitations with the agent on host network mode
		// https://kubernetes.io/docs/concepts/services-networking/windows-networking/#limitations
		cloudwatchAgentServiceEndpoint = "cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local"
	}

	// set protocol by checking cloudwatch agent config for tls setting
	exporterPrefix := http
	isApplicationSignalsEnabled := agentConfig != nil && agentConfig.GetApplicationSignalsMetricsConfig() != nil

	if isApplicationSignalsEnabled {
		if agentConfig.GetApplicationSignalsMetricsConfig().TLS != nil {
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
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: javaInstrumentationImage,
				Env:   getJavaEnvs(isApplicationSignalsEnabled, cloudwatchAgentServiceEndpoint, exporterPrefix, additionalEnvs[TypeJava]),
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(java, limit),
					Requests: getInstrumentationConfigForResource(java, request),
				},
			},
			Python: v1alpha1.Python{
				Image: pythonInstrumentationImage,
				Env:   getPythonEnvs(isApplicationSignalsEnabled, cloudwatchAgentServiceEndpoint, exporterPrefix, additionalEnvs[TypePython]),
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(python, limit),
					Requests: getInstrumentationConfigForResource(python, request),
				},
			},
			DotNet: v1alpha1.DotNet{
				Image: dotNetInstrumentationImage,
				Env:   getDotNetEnvs(isApplicationSignalsEnabled, cloudwatchAgentServiceEndpoint, exporterPrefix, additionalEnvs[TypeDotNet]),
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(dotNet, limit),
					Requests: getInstrumentationConfigForResource(dotNet, request),
				},
			},
			NodeJS: v1alpha1.NodeJS{
				Image: nodeJSInstrumentationImage,
				Env:   getNodeJSEnvs(isApplicationSignalsEnabled, cloudwatchAgentServiceEndpoint, exporterPrefix, additionalEnvs[TypeDotNet]),
				Resources: corev1.ResourceRequirements{
					Limits:   getInstrumentationConfigForResource(nodeJS, limit),
					Requests: getInstrumentationConfigForResource(nodeJS, request),
				},
			},
		},
	}, nil
}

func getJavaEnvs(isAppSignalsEnabled bool, cloudwatchAgentServiceEndpoint, exporterPrefix string, additionalEnvs map[string]string) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
		{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
		{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
	}

	if isAppSignalsEnabled {
		isJavaRuntimeEnabled, ok := os.LookupEnv("AUTO_INSTRUMENTATION_JAVA_RUNTIME_ENABLED")
		if !ok {
			isJavaRuntimeEnabled = "true"
		}
		appSignalsEnvs := []corev1.EnvVar{
			{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
			{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", http, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
			{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)}, //TODO: remove in favor of new name once safe
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: isJavaRuntimeEnabled},
		}
		envs = append(envs, appSignalsEnvs...)
	} else {
		envs = append(envs, corev1.EnvVar{
			Name: "OTEL_TRACES_EXPORTER", Value: "none",
		})
	}

	var jmxEnvs []corev1.EnvVar
	if targetSystems, ok := additionalEnvs[jmx.EnvTargetSystem]; ok {
		jmxEnvs = []corev1.EnvVar{
			{Name: "OTEL_AWS_JMX_EXPORTER_METRICS_ENDPOINT", Value: fmt.Sprintf("%s://%s:4314/v1/metrics", http, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_JMX_TARGET_SYSTEM", Value: targetSystems},
		}
	}
	if len(jmxEnvs) != 0 {
		envs = append(envs, jmxEnvs...)
	}
	return envs
}

func getPythonEnvs(isAppSignalsEnabled bool, cloudwatchAgentServiceEndpoint, exporterPrefix string, additionalEnvs map[string]string) []corev1.EnvVar {
	var envs []corev1.EnvVar
	if isAppSignalsEnabled {
		isPythonRuntimeEnabled, ok := os.LookupEnv("AUTO_INSTRUMENTATION_PYTHON_RUNTIME_ENABLED")
		if !ok {
			isPythonRuntimeEnabled = "true"
		}
		envs = []corev1.EnvVar{
			{Name: "OTEL_AWS_APP_SIGNALS_ENABLED", Value: "true"}, //TODO: remove in favor of new name once safe
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
			{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", http, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
			{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
			{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_AWS_APP_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)}, //TODO: remove in favor of new name once safe
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: isPythonRuntimeEnabled},
			{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
			{Name: "OTEL_PYTHON_DISTRO", Value: "aws_distro"},
			{Name: "OTEL_PYTHON_CONFIGURATOR", Value: "aws_configurator"},
			{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
		}
	}
	return envs
}

func getDotNetEnvs(isAppSignalsEnabled bool, cloudwatchAgentServiceEndpoint, exporterPrefix string, additionalEnvs map[string]string) []corev1.EnvVar {
	var envs []corev1.EnvVar
	if isAppSignalsEnabled {
		isDotNetRuntimeEnabled, ok := os.LookupEnv("AUTO_INSTRUMENTATION_DOTNET_RUNTIME_ENABLED")
		if !ok {
			isDotNetRuntimeEnabled = "true"
		}
		envs = []corev1.EnvVar{
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_RUNTIME_ENABLED", Value: isDotNetRuntimeEnabled},
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
		}
	}
	return envs
}

func getNodeJSEnvs(isAppSignalsEnabled bool, cloudwatchAgentServiceEndpoint, exporterPrefix string, additionalEnvs map[string]string) []corev1.EnvVar {
	var envs []corev1.EnvVar
	if isAppSignalsEnabled {
		envs = []corev1.EnvVar{
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
			{Name: "OTEL_TRACES_SAMPLER_ARG", Value: fmt.Sprintf("endpoint=%s://%s:2000", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_TRACES_SAMPLER", Value: "xray"},
			{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "http/protobuf"},
			{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/traces", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT", Value: fmt.Sprintf("%s://%s:4316/v1/metrics", exporterPrefix, cloudwatchAgentServiceEndpoint)},
			{Name: "OTEL_METRICS_EXPORTER", Value: "none"},
			{Name: "OTEL_LOGS_EXPORTER", Value: "none"},
		}
	}
	return envs
}
