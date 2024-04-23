// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

const (
	NeuronConfigMapName       = "neuron-monitor-config-map"
	NeuronConfigMapVolumeName = "neuron-monitor-config"
	NeuronMonitorJson         = "monitor.json"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := NeuronConfigMapName
	labels := manifestutils.Labels(params.NeuronExp.ObjectMeta, name, params.NeuronExp.Spec.Image, ComponentNeuronExporter, []string{})

	data := map[string]string{
		NeuronMonitorJson: params.NeuronExp.Spec.MonitorConfig,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.NeuronExp.Namespace,
			Labels:      labels,
			Annotations: params.NeuronExp.Annotations,
		},
		Data: data,
	}, nil
}
