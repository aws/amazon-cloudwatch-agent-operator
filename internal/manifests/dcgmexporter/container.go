// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

const (
	configmapMountPath  = "/etc/dcgm-exporter"
	metricsConfigEnvVar = "DCGM_EXPORTER_COLLECTORS"
)

// Container builds a container for the given dcgm exporter.
func Container(cfg config.Config, logger logr.Logger, exporter v1alpha1.DcgmExporter) corev1.Container {
	image := exporter.Spec.Image
	if len(image) == 0 {
		image = cfg.DcgmExporterImage()
	}

	ports := make([]corev1.ContainerPort, 0, len(exporter.Spec.Ports))
	for _, p := range exporter.Spec.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}

	argsMap := exporter.Spec.Args
	if argsMap == nil {
		argsMap = map[string]string{}
	}

	if len(exporter.Spec.TlsConfig) > 0 {
		argsMap["web-config-file"] = fmt.Sprintf("%s/%s", configmapMountPath, DcgmWebConfigYaml)
	}

	// defines the output (sorted) array for final output
	var args []string
	// ensure that the v1alpha1.DcgmExporterSpec.Args are ordered when moved to container.Args,
	// where iterating over a map does not guarantee, so that reconcile will not be fooled by different
	// ordering in args.
	var sortedArgs []string
	for k, v := range argsMap {
		sortedArgs = append(sortedArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	sort.Strings(sortedArgs)
	args = append(args, sortedArgs...)

	var volumeMounts []corev1.VolumeMount
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      DcgmConfigMapVolumeName,
		MountPath: configmapMountPath,
	})
	if len(exporter.Spec.VolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, exporter.Spec.VolumeMounts...)
	}

	var envVars = exporter.Spec.Env
	if exporter.Spec.Env == nil {
		envVars = []corev1.EnvVar{}
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  metricsConfigEnvVar,
		Value: fmt.Sprintf("%s/%s", configmapMountPath, DcgmMetricsIncludedCsv),
	})

	return corev1.Container{
		Name:            ComponentDcgmExporter,
		Image:           image,
		Args:            args,
		Resources:       exporter.Spec.Resources,
		Env:             envVars,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: exporter.Spec.SecurityContext,
	}
}
