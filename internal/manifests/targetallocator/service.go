// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Service(params Params) *corev1.Service {
	name := naming.TAService(params.TargetAllocator.Name)
	labels := manifestutils.Labels(params.TargetAllocator.ObjectMeta, name, params.TargetAllocator.Spec.Image, ComponentAmazonCloudWatchAgentTargetAllocator, nil)
	selector := manifestutils.TASelectorLabels(params.TargetAllocator, ComponentAmazonCloudWatchAgentTargetAllocator)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.TAService(params.TargetAllocator.Name),
			Namespace: params.TargetAllocator.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "targetallocation",
				Port:       80,
				TargetPort: intstr.FromString("http"),
			}},
			IPFamilies:     params.TargetAllocator.Spec.IpFamilies,
			IPFamilyPolicy: params.TargetAllocator.Spec.IpFamilyPolicy,
		},
	}
}
