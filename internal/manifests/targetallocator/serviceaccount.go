// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

const targetAllocatorServiceAcctName = "target-allocator-service-acct"

// ServiceAccountName returns the name of the existing or self-provisioned service account to use for the given instance.
func ServiceAccountName(instance v1alpha1.AmazonCloudWatchAgent) string {
	if len(instance.Spec.TargetAllocator.ServiceAccount) == 0 {
		return targetAllocatorServiceAcctName
	}

	return instance.Spec.TargetAllocator.ServiceAccount
}

// ServiceAccount returns the service account for the given instance.
func ServiceAccount(params manifests.Params) *corev1.ServiceAccount {
	labels := Labels(params.OtelCol, targetAllocatorServiceAcctName)

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        targetAllocatorServiceAcctName,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
	}
}
