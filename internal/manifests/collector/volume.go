// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package collector handles the OpenTelemetry Collector.
package collector

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// Volumes builds the volumes for the given instance, including the config map volume.
func Volumes(cfg config.Config, otelcol v1alpha1.AmazonCloudWatchAgent) []corev1.Volume {
	volumes := []corev1.Volume{{
		Name: naming.ConfigMapVolume(),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: naming.ConfigMap(otelcol.Name)},
				Items: []corev1.KeyToPath{{
					Key:  cfg.CollectorConfigMapEntry(),
					Path: cfg.CollectorConfigMapEntry(),
				}},
			},
		},
	}}

	if len(otelcol.Spec.Volumes) > 0 {
		volumes = append(volumes, otelcol.Spec.Volumes...)
	}

	if len(otelcol.Spec.ConfigMaps) > 0 {
		for keyCfgMap := range otelcol.Spec.ConfigMaps {
			volumes = append(volumes, corev1.Volume{
				Name: naming.ConfigMapExtra(otelcol.Spec.ConfigMaps[keyCfgMap].Name),
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: otelcol.Spec.ConfigMaps[keyCfgMap].Name,
						},
					},
				},
			})
		}
	}

	return volumes
}
