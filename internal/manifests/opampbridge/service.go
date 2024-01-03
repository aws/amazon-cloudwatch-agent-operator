// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Service(params manifests.Params) *corev1.Service {
	name := naming.OpAMPBridgeService(params.OpAMPBridge.Name)
	labels := manifestutils.Labels(params.OpAMPBridge.ObjectMeta, name, params.OpAMPBridge.Spec.Image, ComponentOpAMPBridge, []string{})
	selector := manifestutils.SelectorLabels(params.OpAMPBridge.ObjectMeta, ComponentOpAMPBridge)

	ports := []corev1.ServicePort{{
		Name:       "opamp-bridge",
		Port:       80,
		TargetPort: intstr.FromInt(8080),
	}}

	ports = append(params.OpAMPBridge.Spec.Ports, ports...)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: params.OpAMPBridge.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    ports,
		},
	}
}
