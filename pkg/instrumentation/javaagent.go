// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	envJavaToolsOptions       = "JAVA_TOOL_OPTIONS"
	javaJVMArgument           = " -javaagent:/otel-auto-instrumentation-java/javaagent.jar"
	javaInitContainerName     = initContainerName + "-java"
	javaVolumeName            = volumeName + "-java"
	javaInstrMountPath        = "/otel-auto-instrumentation-java"
	javaInstrMountPathWindows = "\\otel-auto-instrumentation-java"
)

var (
	javaCommandLinux   = []string{"cp", "/javaagent.jar", javaInstrMountPath + "/javaagent.jar"}
	javaCommandWindows = []string{"CMD", "/c", "copy", "javaagent.jar", javaInstrMountPathWindows}
)

func injectJavaagent(javaSpec v1alpha1.Java, pod corev1.Pod, index int, allEnvs []corev1.EnvVar) (corev1.Pod, error) {
	container := &pod.Spec.Containers[index]

	err := validateContainerEnv(container.Env, envJavaToolsOptions)
	if err != nil {
		return pod, err
	}

	// Check if ADOT SDK should be injected based on all environment variables and security context
	if !shouldInjectADOTSDK(allEnvs, pod, container) {
		return pod, fmt.Errorf("ADOT Java SDK injection skipped due to incompatible OTel configuration")
	}

	// inject Java instrumentation spec env vars with validation
	for _, env := range javaSpec.Env {
		if shouldInjectEnvVar(allEnvs, env.Name) {
			container.Env = append(container.Env, env)
		}
	}

	idx := getIndexOfEnv(container.Env, envJavaToolsOptions)
	if idx == -1 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envJavaToolsOptions,
			Value: javaJVMArgument,
		})
	} else {
		container.Env[idx].Value = container.Env[idx].Value + javaJVMArgument
	}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      javaVolumeName,
		MountPath: javaInstrMountPath,
	})

	// We just inject Volumes and init containers for the first processed container.
	if isInitContainerMissing(pod, javaInitContainerName) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: javaVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: volumeSize(javaSpec.VolumeSizeLimit),
				},
			}})

		command := javaCommandLinux
		if isWindowsPod(pod) {
			command = javaCommandWindows
		}

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:      javaInitContainerName,
			Image:     javaSpec.Image,
			Command:   command,
			Resources: javaSpec.Resources,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      javaVolumeName,
				MountPath: javaInstrMountPath,
			}},
		})
	}

	return pod, err
}
