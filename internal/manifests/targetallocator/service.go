// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Service(params manifests.Params) *corev1.Service {
	version := strings.Split(params.OtelCol.Spec.TargetAllocator.Image, ":")
	labels := Labels(params.OtelCol, naming.TAService())
	if len(version) > 1 {
		labels["app.kubernetes.io/version"] = version[len(version)-1]
	} else {
		labels["app.kubernetes.io/version"] = "latest"
	}

	selector := Labels(params.OtelCol, naming.TAService())

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.TAService(),
			Namespace: params.OtelCol.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "targetallocation",
				Port:       naming.TargetAllocatorServicePort,
				TargetPort: intstr.FromInt32(naming.TargetAllocatorContainerPort),
			}},
		},
	}
}
