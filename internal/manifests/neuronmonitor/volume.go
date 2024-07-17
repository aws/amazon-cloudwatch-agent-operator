// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
)

// Volumes builds the volumes for the given instance, including the config map volume.
func Volumes(exporter v1beta1.NeuronMonitor) []corev1.Volume {
	var volumes []corev1.Volume
	if len(exporter.Spec.Volumes) > 0 {
		volumes = append(volumes, exporter.Spec.Volumes...)
	}

	//configmap volume
	volumes = append(volumes, corev1.Volume{
		Name: NeuronConfigMapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: NeuronConfigMapName,
				},
			},
		},
	})
	return volumes
}
