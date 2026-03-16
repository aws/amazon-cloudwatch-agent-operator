// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

const (
	NodeExporterConfigMapName       = "node-exporter-config-map"
	NodeExporterConfigMapVolumeName = "node-exporter-config"
	NodeExporterWebConfigYaml       = "web.yml"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := NodeExporterConfigMapName
	labels := manifestutils.Labels(params.NodeExp.ObjectMeta, name, params.NodeExp.Spec.Image, ComponentNodeExporter, []string{})

	data := map[string]string{}
	if len(params.NodeExp.Spec.TlsConfig) > 0 {
		data[NodeExporterWebConfigYaml] = params.NodeExp.Spec.TlsConfig
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.NodeExp.Namespace,
			Labels:      labels,
			Annotations: params.NodeExp.Annotations,
		},
		Data: data,
	}, nil
}
