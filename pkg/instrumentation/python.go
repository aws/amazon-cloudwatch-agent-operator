// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	envPythonPath                      = "PYTHONPATH"
	envOtelTracesExporter              = "OTEL_TRACES_EXPORTER"
	envOtelMetricsExporter             = "OTEL_METRICS_EXPORTER"
	envOtelExporterOTLPTracesProtocol  = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
	envOtelExporterOTLPMetricsProtocol = "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"
	pythonPathPrefix                   = "/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation"
	pythonPathSuffix                   = "/otel-auto-instrumentation-python"
	pythonInstrMountPath               = "/otel-auto-instrumentation-python"
	pythonVolumeName                   = volumeName + "-python"
	pythonInitContainerName            = initContainerName + "-python"
)

func injectPythonSDK(pythonSpec v1alpha1.Python, pod corev1.Pod, index int, allEnvs []corev1.EnvVar) (corev1.Pod, error) {
	container := &pod.Spec.Containers[index]

	err := validateContainerEnv(container.Env, envPythonPath)
	if err != nil {
		return pod, err
	}

	// Check if ADOT SDK should be injected based on all environment variables and security context
	if !shouldInjectADOTSDK(allEnvs, pod, container) {
		return pod, fmt.Errorf("ADOT Python SDK injection skipped due to incompatible OTel configuration")
	}

	// inject Python instrumentation spec env vars with validation
	for _, env := range pythonSpec.Env {
		if shouldInjectEnvVar(allEnvs, env.Name) {
			container.Env = append(container.Env, env)
		}
	}

	idx := getIndexOfEnv(container.Env, envPythonPath)
	if idx == -1 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envPythonPath,
			Value: fmt.Sprintf("%s:%s", pythonPathPrefix, pythonPathSuffix),
		})
	} else if idx > -1 {
		container.Env[idx].Value = fmt.Sprintf("%s:%s:%s", pythonPathPrefix, container.Env[idx].Value, pythonPathSuffix)
	}

	// Set OTEL_TRACES_EXPORTER to otlp exporter if not set by user and validation allows
	if shouldInjectEnvVar(container.Env, envOtelTracesExporter) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envOtelTracesExporter,
			Value: "otlp",
		})
	}

	// Set OTEL_EXPORTER_OTLP_TRACES_PROTOCOL to http/protobuf if not set by user and validation allows
	if shouldInjectEnvVar(container.Env, envOtelExporterOTLPTracesProtocol) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envOtelExporterOTLPTracesProtocol,
			Value: "http/protobuf",
		})
	}

	// Set OTEL_METRICS_EXPORTER to otlp exporter if not set by user and validation allows
	if shouldInjectEnvVar(container.Env, envOtelMetricsExporter) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envOtelMetricsExporter,
			Value: "otlp",
		})
	}

	// Set OTEL_EXPORTER_OTLP_METRICS_PROTOCOL to http/protobuf if not set by user and validation allows
	if shouldInjectEnvVar(container.Env, envOtelExporterOTLPMetricsProtocol) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envOtelExporterOTLPMetricsProtocol,
			Value: "http/protobuf",
		})
	}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      pythonVolumeName,
		MountPath: pythonInstrMountPath,
	})

	// We just inject Volumes and init containers for the first processed container.
	if isInitContainerMissing(pod, pythonInitContainerName) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: pythonVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: volumeSize(pythonSpec.VolumeSizeLimit),
				},
			}})

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:      pythonInitContainerName,
			Image:     pythonSpec.Image,
			Command:   []string{"cp", "-a", "/autoinstrumentation/.", pythonInstrMountPath},
			Resources: pythonSpec.Resources,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      pythonVolumeName,
				MountPath: pythonInstrMountPath,
			}},
		})
	}
	return pod, nil
}
