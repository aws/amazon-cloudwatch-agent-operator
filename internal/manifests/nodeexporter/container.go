// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

const (
	configmapMountPath = "/etc/node-exporter"
)

// Container builds a container for the given node exporter.
func Container(cfg config.Config, logger logr.Logger, exporter v1alpha1.NodeExporter) corev1.Container {
	image := exporter.Spec.Image
	if len(image) == 0 {
		image = cfg.NodeExporterImage()
	}

	// Default args for node-exporter; copy to avoid mutating the spec
	argsMap := make(map[string]string, len(exporter.Spec.Args))
	for k, v := range exporter.Spec.Args {
		argsMap[k] = v
	}
	if _, ok := argsMap["path.rootfs"]; !ok {
		argsMap["path.rootfs"] = "/host/root"
	}
	if _, ok := argsMap["path.sysfs"]; !ok {
		argsMap["path.sysfs"] = "/host/sys"
	}
	if _, ok := argsMap["path.procfs"]; !ok {
		argsMap["path.procfs"] = "/host/proc"
	}
	if _, ok := argsMap["web.listen-address"]; !ok {
		argsMap["web.listen-address"] = ":9100"
	}

	if len(exporter.Spec.TlsConfig) > 0 {
		argsMap["web.config.file"] = fmt.Sprintf("%s/%s", configmapMountPath, NodeExporterWebConfigYaml)
	}

	// ensure args are sorted so reconcile is not fooled by different map iteration ordering
	var args []string
	for k, v := range argsMap {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	sort.Strings(args)

	// Ports: 9100 with hostPort
	ports := []corev1.ContainerPort{{
		Name:          "metrics",
		ContainerPort: 9100,
		HostPort:      9100,
		Protocol:      corev1.ProtocolTCP,
	}}
	if len(exporter.Spec.Ports) > 0 {
		ports = make([]corev1.ContainerPort, 0, len(exporter.Spec.Ports))
		for _, p := range exporter.Spec.Ports {
			ports = append(ports, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.Port,
				HostPort:      p.Port,
				Protocol:      p.Protocol,
			})
		}
	}

	var volumeMounts []corev1.VolumeMount
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      NodeExporterConfigMapVolumeName,
		MountPath: configmapMountPath,
		ReadOnly:  true,
	})
	if len(exporter.Spec.VolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, exporter.Spec.VolumeMounts...)
	}

	return corev1.Container{
		Name:            ComponentNodeExporter,
		Image:           image,
		Args:            args,
		Resources:       exporter.Spec.Resources,
		Env:             exporter.Spec.Env,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: exporter.Spec.SecurityContext,
	}
}
