// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// Volumes builds the volumes for the given instance, including the config map volume.
func Volumes(cfg config.Config, opampBridge v1alpha1.OpAMPBridge) []corev1.Volume {
	volumes := []corev1.Volume{{
		Name: naming.OpAMPBridgeConfigMapVolume(),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: naming.OpAMPBridgeConfigMap(opampBridge.Name)},
				Items: []corev1.KeyToPath{
					{
						Key:  cfg.OperatorOpAMPBridgeConfigMapEntry(),
						Path: cfg.OperatorOpAMPBridgeConfigMapEntry(),
					},
				},
			},
		},
	}}

	return volumes
}
