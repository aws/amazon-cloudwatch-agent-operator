// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ta "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator/adapters"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := naming.ConfigMap(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, []string{})

	replacedConf, err := ReplaceConfig(params.OtelCol)
	if err != nil {
		params.Log.V(2).Info("failed to update prometheus config to use sharded targets: ", "err", err)
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
		Data: map[string]string{
			"cwagentconfig.json": replacedConf,
		},
	}, nil
}

func PrometheusConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := naming.PrometheusConfigMap(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, []string{})

	replacedPrometheusConf, err := ReplacePrometheusConfig(params.OtelCol)
	if err != nil {
		params.Log.V(2).Info("failed to update prometheus config to use sharded targets: ", "err", err)
		return nil, err
	}

	prometheusReceiverConfig, err := ta.GetPromConfig(replacedPrometheusConf)
	if err != nil {
		return nil, err
	}

	prometheusConfigYAML, err := yaml.Marshal(prometheusReceiverConfig)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
		Data: map[string]string{
			"prometheus.yaml": string(prometheusConfigYAML),
		},
	}, nil
}
