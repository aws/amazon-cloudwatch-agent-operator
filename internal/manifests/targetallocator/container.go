// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)
// This has to be a constant so that TA container code can access it as well
const TACertMountPath = "/etc/amazon-cloudwatch-target-allocator-cert"
// Container builds a container for the given TargetAllocator.
func Container(cfg config.Config, logger logr.Logger, otelcol v1alpha1.AmazonCloudWatchAgent) corev1.Container {
	image := otelcol.Spec.TargetAllocator.Image
	if len(image) == 0 {
		image = cfg.TargetAllocatorImage()
	}

	ports := make([]corev1.ContainerPort, 0)
	ports = append(ports, corev1.ContainerPort{
		Name:          "https",
		ContainerPort: naming.TargetAllocatorContainerPort,
		Protocol:      corev1.ProtocolTCP,
	})

	volumeMounts := []corev1.VolumeMount{{
		Name:      naming.TAConfigMapVolume(),
		MountPath: "/conf",
	}, {
		Name:      naming.TASecretVolume(),
		MountPath: TACertMountPath,
		ReadOnly:  true,
	},
	}

	var envVars = otelcol.Spec.TargetAllocator.Env
	if otelcol.Spec.TargetAllocator.Env == nil {
		envVars = []corev1.EnvVar{}
	}

	idx := -1
	for i := range envVars {
		if envVars[i].Name == "OTELCOL_NAMESPACE" {
			idx = i
		}
	}
	if idx == -1 {
		envVars = append(envVars, corev1.EnvVar{
			Name: "OTELCOL_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		})
	}

	var args []string
	if otelcol.Spec.TargetAllocator.PrometheusCR.Enabled {
		args = append(args, "--enable-prometheus-cr-watcher")
	}

	return corev1.Container{
		Name:         naming.TAContainer(),
		Image:        image,
		Ports:        ports,
		Env:          envVars,
		VolumeMounts: volumeMounts,
		Resources:    otelcol.Spec.TargetAllocator.Resources,
		Args:         args,
	}
}
