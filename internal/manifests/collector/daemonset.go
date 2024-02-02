// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// DaemonSet builds the deployment for the given instance.
func DaemonSet(params manifests.Params) *appsv1.DaemonSet {
	name := naming.Collector(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, params.Config.LabelsFilter())

	annotations := Annotations(params.OtelCol)
	podAnnotations := PodAnnotations(params.OtelCol)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.Collector(params.OtelCol.Name),
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.OtelCol.ObjectMeta, ComponentAmazonCloudWatchAgent),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(params.OtelCol),
					InitContainers:     params.OtelCol.Spec.InitContainers,
					Containers:         append(params.OtelCol.Spec.AdditionalContainers, Container(params.Config, params.Log, params.OtelCol, true)),
					Volumes:            Volumes(params.Config, params.OtelCol),
					Tolerations:        params.OtelCol.Spec.Tolerations,
					NodeSelector:       params.OtelCol.Spec.NodeSelector,
					HostNetwork:        params.OtelCol.Spec.HostNetwork,
					DNSPolicy:          getDNSPolicy(params.OtelCol),
					SecurityContext:    params.OtelCol.Spec.PodSecurityContext,
					PriorityClassName:  params.OtelCol.Spec.PriorityClassName,
					Affinity:           params.OtelCol.Spec.Affinity,
				},
			},
			UpdateStrategy: params.OtelCol.Spec.UpdateStrategy,
		},
	}
}
