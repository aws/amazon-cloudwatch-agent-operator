// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// ServiceAccountName returns the name of the existing or self-provisioned service account to use for the given instance.
func ServiceAccountName(instance v1alpha1.AmazonCloudWatchAgent) string {
	if len(instance.Spec.TargetAllocator.ServiceAccount) == 0 {
		return naming.ServiceAccount(instance.Name)
	}

	return instance.Spec.TargetAllocator.ServiceAccount
}

// ServiceAccount returns the service account for the given instance.
func ServiceAccount(params manifests.Params) *corev1.ServiceAccount {
	name := naming.TargetAllocatorServiceAccount(params.OtelCol.Name)
	labels := Labels(params.OtelCol, name)

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
	}
}
