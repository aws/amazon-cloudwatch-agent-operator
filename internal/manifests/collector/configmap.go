// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func ConfigMaps(params manifests.Params) ([]*corev1.ConfigMap, error) {
	var configmaps []*corev1.ConfigMap

	name := naming.ConfigMap(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, []string{})

	params.Log.V(2).Info("failed to update config: ")
	replacedConf, err := ReplaceConfig(params.OtelCol)
	if err != nil {
		params.Log.V(2).Info("failed to update config: ", "err", err)
		return nil, err
	}

	configmaps = append(configmaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
		Data: map[string]string{
			params.Config.CollectorConfigMapEntry(): replacedConf,
		},
	})

	if !params.OtelCol.Spec.Prometheus.IsEmpty() {
		promName := naming.PrometheusConfigMap(params.OtelCol.Name)
		promLabels := manifestutils.Labels(params.OtelCol.ObjectMeta, promName, "", ComponentAmazonCloudWatchAgent, []string{})

		replacedPrometheusConf, err := ReplacePrometheusConfig(params.OtelCol)
		if err != nil {
			params.Log.V(2).Info("failed to update prometheus config to use sharded targets: ", "err", err)
			return nil, err
		}

		if !params.OtelCol.Spec.TargetAllocator.Enabled {
			replacedPrometheusConfig, err := adapters.ConfigFromString(replacedPrometheusConf)
			if err != nil {
				return nil, err
			}

			replacedPrometheusConfProp, ok := replacedPrometheusConfig["config"]
			if !ok {
				return nil, fmt.Errorf("no prometheusConfig available as part of the configuration")
			}

			replacedPrometheusConfPropYAML, err := yaml.Marshal(replacedPrometheusConfProp)
			if err != nil {
				return nil, err
			}

			replacedPrometheusConf = string(replacedPrometheusConfPropYAML)
		}

		configmaps = append(configmaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        promName,
				Namespace:   params.OtelCol.Namespace,
				Labels:      promLabels,
				Annotations: params.OtelCol.Annotations,
			},
			Data: map[string]string{
				params.Config.PrometheusConfigMapEntry(): replacedPrometheusConf,
			},
		})
	}

	return configmaps, nil
}
