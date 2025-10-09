// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	envNodeOptions          = "NODE_OPTIONS"
	nodeRequireArgument     = " --require /otel-auto-instrumentation-nodejs/autoinstrumentation.js"
	nodejsInitContainerName = initContainerName + "-nodejs"
	nodejsVolumeName        = volumeName + "-nodejs"
	nodejsInstrMountPath    = "/otel-auto-instrumentation-nodejs"
)

func injectNodeJSSDK(nodeJSSpec v1alpha1.NodeJS, pod corev1.Pod, index int, allEnvs []corev1.EnvVar) (corev1.Pod, error) {
	container := &pod.Spec.Containers[index]

	err := validateContainerEnv(container.Env, envNodeOptions)
	if err != nil {
		return pod, err
	}

	// Check if ADOT SDK should be injected based on all environment variables and security context
	if !shouldInjectADOTSDK(allEnvs, pod, container) {
		return pod, fmt.Errorf("ADOT NodeJs SDK injection skipped due to incompatible OTel configuration")
	}

	// inject NodeJS instrumentation spec env vars with validation
	for _, env := range nodeJSSpec.Env {
		if shouldInjectEnvVar(allEnvs, env.Name) {
			container.Env = append(container.Env, env)
		}
	}

	idx := getIndexOfEnv(container.Env, envNodeOptions)
	if idx == -1 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envNodeOptions,
			Value: nodeRequireArgument,
		})
	} else if idx > -1 {
		container.Env[idx].Value = container.Env[idx].Value + nodeRequireArgument
	}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      nodejsVolumeName,
		MountPath: nodejsInstrMountPath,
	})

	// We just inject Volumes and init containers for the first processed container
	if isInitContainerMissing(pod, nodejsInitContainerName) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: nodejsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: volumeSize(nodeJSSpec.VolumeSizeLimit),
				},
			}})

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:      nodejsInitContainerName,
			Image:     nodeJSSpec.Image,
			Command:   []string{"cp", "-r", "/autoinstrumentation/.", nodejsInstrMountPath},
			Resources: nodeJSSpec.Resources,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      nodejsVolumeName,
				MountPath: nodejsInstrMountPath,
			}},
		})
	}
	return pod, nil
}
