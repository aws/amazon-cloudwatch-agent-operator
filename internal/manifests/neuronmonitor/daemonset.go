// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

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
	name := naming.Collector(params.NeuronExp.Name)
	if len(name) == 0 {
		name = ComponentNeuronExporter
	}
	labels := manifestutils.Labels(params.NeuronExp.ObjectMeta, name, params.NeuronExp.Spec.Image, ComponentNeuronExporter, params.Config.LabelsFilter())

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.NeuronExp.Name,
			Namespace:   params.NeuronExp.Namespace,
			Labels:      labels,
			Annotations: Annotations(params.NeuronExp),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.NeuronExp.ObjectMeta, ComponentNeuronExporter),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(params.NeuronExp),
					Containers:         []corev1.Container{Container(params.Config, params.Log, params.NeuronExp)},
					Volumes:            Volumes(params.NeuronExp),
					Tolerations:        params.NeuronExp.Spec.Tolerations,
					NodeSelector:       params.NeuronExp.Spec.NodeSelector,
					Affinity:           params.NeuronExp.Spec.Affinity,
				},
			},
		},
	}
}
