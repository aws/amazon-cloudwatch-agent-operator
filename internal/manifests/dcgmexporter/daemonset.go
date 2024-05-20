// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

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
	name := naming.Collector(params.DcgmExp.Name)
	if len(name) == 0 {
		name = ComponentDcgmExporter
	}
	labels := manifestutils.Labels(params.DcgmExp.ObjectMeta, name, params.DcgmExp.Spec.Image, ComponentDcgmExporter, params.Config.LabelsFilter())

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.DcgmExp.Name,
			Namespace:   params.DcgmExp.Namespace,
			Labels:      labels,
			Annotations: Annotations(params.DcgmExp),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.DcgmExp.ObjectMeta, ComponentDcgmExporter),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(params.DcgmExp),
					Containers:         []corev1.Container{Container(params.Config, params.Log, params.DcgmExp)},
					Volumes:            Volumes(params.DcgmExp),
					Tolerations:        params.DcgmExp.Spec.Tolerations,
					NodeSelector:       params.DcgmExp.Spec.NodeSelector,
					Affinity:           params.DcgmExp.Spec.Affinity,
				},
			},
		},
	}
}
