// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package collector handles the CloudWatch Agent.
package collector

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/naming"
)

// Volumes builds the volumes for the given instance, including the config map volume.
func Volumes(cfg config.Config, otelcol v1alpha1.AmazonCloudWatchAgent) []corev1.Volume {
	volumes := []corev1.Volume{{
		Name: naming.ConfigMapVolume(),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: naming.ConfigMap(otelcol)},
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

	return volumes
}
