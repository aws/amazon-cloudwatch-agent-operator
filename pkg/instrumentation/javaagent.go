// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	envJavaToolsOptions   = "JAVA_TOOL_OPTIONS"
	javaJVMArgument       = " -javaagent:/otel-auto-instrumentation-java/javaagent.jar"
	javaInitContainerName = initContainerName + "-java"
	javaVolumeName        = volumeName + "-java"
	javaInstrMountPath    = "/otel-auto-instrumentation-java"
)

func injectJavaagent(javaSpec v1alpha1.Java, pod corev1.Pod, index int) (corev1.Pod, error) {
	// caller checks if there is at least one container.
	container := &pod.Spec.Containers[index]

	err := validateContainerEnv(container.Env, envJavaToolsOptions)
	if err != nil {
		return pod, err
	}

	// inject Java instrumentation spec env vars.
	for _, env := range javaSpec.Env {
		idx := getIndexOfEnv(container.Env, env.Name)
		if idx == -1 {
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

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:      javaInitContainerName,
			Image:     javaSpec.Image,
			Command:   []string{"cp", "/javaagent.jar", javaInstrMountPath + "/javaagent.jar"},
			Resources: javaSpec.Resources,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      javaVolumeName,
				MountPath: javaInstrMountPath,
			}},
		})
	}
	return pod, err
}
