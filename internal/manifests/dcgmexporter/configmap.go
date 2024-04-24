// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

const (
	DcgmConfigMapName       = "dcgm-exporter-config-map"
	DcgmConfigMapVolumeName = "dcgm-config"
	DcgmMetricsIncludedCsv  = "dcp-metrics-included.csv"
	DcgmWebConfigYaml       = "web-config.yaml"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := DcgmConfigMapName
	labels := manifestutils.Labels(params.DcgmExp.ObjectMeta, name, params.DcgmExp.Spec.Image, ComponentDcgmExporter, []string{})

	data := map[string]string{
		DcgmMetricsIncludedCsv: params.DcgmExp.Spec.MetricsConfig,
	}
	if len(params.DcgmExp.Spec.TlsConfig) > 0 {
		data[DcgmWebConfigYaml] = params.DcgmExp.Spec.TlsConfig
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.DcgmExp.Namespace,
			Labels:      labels,
			Annotations: params.DcgmExp.Annotations,
		},
		Data: data,
	}, nil
}
