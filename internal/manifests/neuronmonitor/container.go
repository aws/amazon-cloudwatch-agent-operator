// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

const (
	configmapMountPath = "/etc/neuron-monitor-config"
)

// Container builds a container for the given neuron monitor exporter.
func Container(cfg config.Config, logger logr.Logger, exporter v1alpha1.NeuronMonitor) corev1.Container {
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
	// Sort map keys to ensure deterministic order
	var keys []string
	for k := range argsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--"+k, argsMap[k])
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

	// Add health probes for Neuron Monitor using utility functions
	var probePort intstr.IntOrString
	if len(ports) > 0 {
		probePort = intstr.FromInt32(ports[0].ContainerPort)
	} else {
		probePort = intstr.FromInt(10259) // Default Neuron monitor health port
	}

	// Create custom probe config for Neuron Monitor (needs more time to start)
	// Use static variables to avoid creating new memory addresses on each call
	initialDelaySeconds := int32(90)
	timeoutSeconds := int32(20)
	failureThreshold := int32(5)

	customProbeConfig := &v1alpha1.Probe{
		InitialDelaySeconds: &initialDelaySeconds,
		TimeoutSeconds:      &timeoutSeconds,
		FailureThreshold:    &failureThreshold,
	}

	livenessProbe := manifestutils.CreateLivenessProbe("/healthz", probePort, customProbeConfig)
	readinessProbe := manifestutils.CreateReadinessProbe("/healthz", probePort, customProbeConfig)

	// Set HTTPS scheme for Neuron Monitor probes
	if livenessProbe.HTTPGet != nil {
		livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	}
	if readinessProbe.HTTPGet != nil {
		readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
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
		LivenessProbe:   livenessProbe,
		ReadinessProbe:  readinessProbe,
	}
}
