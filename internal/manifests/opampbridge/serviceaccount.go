// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// ServiceAccountName returns the name of the existing or self-provisioned service account to use for the given instance.
func ServiceAccountName(instance v1alpha1.OpAMPBridge) string {
	if len(instance.Spec.ServiceAccount) == 0 {
		return naming.OpAMPBridgeServiceAccount(instance.Name)
	}
	return instance.Spec.ServiceAccount
}

// ServiceAccount returns the service account for the given instance.
func ServiceAccount(params manifests.Params) *corev1.ServiceAccount {
	name := naming.OpAMPBridgeServiceAccount(params.OpAMPBridge.Name)
	labels := manifestutils.Labels(params.OpAMPBridge.ObjectMeta, name, params.OpAMPBridge.Spec.Image, ComponentOpAMPBridge, []string{})

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OpAMPBridge.Namespace,
			Labels:      labels,
			Annotations: params.OpAMPBridge.Annotations,
		},
	}
}
