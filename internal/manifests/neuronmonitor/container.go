// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

const (
	configmapMountPath = "/etc/neuron-monitor-config"
)

// Container builds a container for the given neuron monitor exporter.
func Container(cfg config.Config, logger logr.Logger, exporter v1beta1.NeuronMonitor) corev1.Container {
	image := exporter.Spec.Image
	if len(image) == 0 {
		image = cfg.NeuronMonitorImage()
	}

	ports := make([]corev1.ContainerPort, 0, len(exporter.Spec.Ports))
	for _, p := range exporter.Spec.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}

	command := exporter.Spec.Command

	argsMap := exporter.Spec.Args
	if argsMap == nil {
		argsMap = map[string]string{}
	}
	var args []string
	for k, v := range argsMap {
		args = append(args, "--"+k, v)
	}
	args = append(args, "--neuron-monitor-config", fmt.Sprintf("%s/%s", configmapMountPath, NeuronMonitorJson))

	var volumeMounts []corev1.VolumeMount
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      NeuronConfigMapVolumeName,
		MountPath: configmapMountPath,
	})
	if len(exporter.Spec.VolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, exporter.Spec.VolumeMounts...)
	}

	var envVars = exporter.Spec.Env
	if exporter.Spec.Env == nil {
		envVars = []corev1.EnvVar{}
	}

	return corev1.Container{
		Name:            ComponentNeuronExporter,
		Image:           image,
		Command:         command,
		Args:            args,
		SecurityContext: exporter.Spec.SecurityContext,
		Resources:       exporter.Spec.Resources,
		Env:             envVars,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
	}
}
