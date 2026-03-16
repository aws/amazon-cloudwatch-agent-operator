// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

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
	name := naming.Collector(params.NodeExp.Name)
	if len(name) == 0 {
		name = ComponentNodeExporter
	}
	labels := manifestutils.Labels(params.NodeExp.ObjectMeta, name, params.NodeExp.Spec.Image, ComponentNodeExporter, params.Config.LabelsFilter())

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.NodeExp.Name,
			Namespace:   params.NodeExp.Namespace,
			Labels:      labels,
			Annotations: Annotations(params.NodeExp),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.NodeExp.ObjectMeta, ComponentNodeExporter),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(params.NodeExp),
					Containers:         []corev1.Container{Container(params.Config, params.Log, params.NodeExp)},
					Volumes:            Volumes(params.NodeExp),
					Tolerations:        params.NodeExp.Spec.Tolerations,
					NodeSelector:       params.NodeExp.Spec.NodeSelector,
					Affinity:           params.NodeExp.Spec.Affinity,
				},
			},
		},
	}
}
