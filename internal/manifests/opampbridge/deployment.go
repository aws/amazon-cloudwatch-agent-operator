// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// Deployment builds the deployment for the given instance.
func Deployment(params manifests.Params) *appsv1.Deployment {
	name := naming.OpAMPBridge(params.OpAMPBridge.Name)
	labels := manifestutils.Labels(params.OpAMPBridge.ObjectMeta, name, params.OpAMPBridge.Spec.Image, ComponentOpAMPBridge, params.Config.LabelsFilter())

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OpAMPBridge.Namespace,
			Labels:      labels,
			Annotations: params.OpAMPBridge.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: params.OpAMPBridge.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.OpAMPBridge.ObjectMeta, ComponentOpAMPBridge),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: params.OpAMPBridge.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:        ServiceAccountName(params.OpAMPBridge),
					Containers:                []corev1.Container{Container(params.Config, params.Log, params.OpAMPBridge)},
					Volumes:                   Volumes(params.Config, params.OpAMPBridge),
					DNSPolicy:                 getDNSPolicy(params.OpAMPBridge),
					HostNetwork:               params.OpAMPBridge.Spec.HostNetwork,
					Tolerations:               params.OpAMPBridge.Spec.Tolerations,
					NodeSelector:              params.OpAMPBridge.Spec.NodeSelector,
					SecurityContext:           params.OpAMPBridge.Spec.PodSecurityContext,
					PriorityClassName:         params.OpAMPBridge.Spec.PriorityClassName,
					Affinity:                  params.OpAMPBridge.Spec.Affinity,
					TopologySpreadConstraints: params.OpAMPBridge.Spec.TopologySpreadConstraints,
				},
			},
		},
	}
}
