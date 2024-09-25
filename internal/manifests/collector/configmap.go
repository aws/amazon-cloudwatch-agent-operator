// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func ConfigMaps(params manifests.Params) ([]*corev1.ConfigMap, error) {
	name := naming.ConfigMap(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, []string{})

	replacedConf, err := ReplaceConfig(params.OtelCol)
	if err != nil {
		params.Log.V(2).Info("failed to update config: ", "err", err)
		return nil, err
	}

	cms := []*corev1.ConfigMap{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
		Data: map[string]string{
			"cwagentconfig.json": replacedConf,
		},
	}}

	if params.OtelCol.Spec.OtelConfig != "" {
		otelName := naming.ConfigMapOtelCollector(params.OtelCol.Name)

		replacedOtelConfig, err := ReplaceOtelConfig(params.OtelCol)
		if err != nil {
			params.Log.V(2).Info("failed to update otel config: ", "err", err)
			return nil, err
		}

		cms = append(cms, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        otelName,
				Namespace:   params.OtelCol.Namespace,
				Labels:      labels,
				Annotations: params.OtelCol.Annotations,
			},
			Data: map[string]string{
				"cwagentotelconfig.yaml": replacedOtelConfig,
			},
		})
	}
	return cms, nil
}
