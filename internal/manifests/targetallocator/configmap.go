// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

const (
	targetAllocatorFilename = "targetallocator.yaml"
)

func ConfigMap(params Params) (*corev1.ConfigMap, error) {
	instance := params.TargetAllocator
	name := naming.TAConfigMap(instance.Name)
	labels := manifestutils.Labels(instance.ObjectMeta, name, params.TargetAllocator.Spec.Image, ComponentAmazonCloudWatchAgentTargetAllocator, nil)
	taSpec := instance.Spec

	taConfig := make(map[interface{}]interface{})

	taConfig["collector_selector"] = metav1.LabelSelector{
		MatchLabels: manifestutils.SelectorLabels(params.Collector.ObjectMeta, collector.ComponentAmazonCloudWatchAgent),
	}

	// Add scrape configs if present
	if instance.Spec.ScrapeConfigs != nil && len(instance.Spec.ScrapeConfigs) > 0 {
		taConfig["config"] = map[string]interface{}{
			"scrape_configs": instance.Spec.ScrapeConfigs,
		}
	}

	if len(taSpec.AllocationStrategy) > 0 {
		taConfig["allocation_strategy"] = taSpec.AllocationStrategy
	} else {
		taConfig["allocation_strategy"] = v1alpha1.TargetAllocatorAllocationStrategyConsistentHashing
	}
	taConfig["filter_strategy"] = taSpec.FilterStrategy

	if taSpec.PrometheusCR.Enabled {
		prometheusCRConfig := map[interface{}]interface{}{
			"enabled": true,
		}
		if taSpec.PrometheusCR.ScrapeInterval.Size() > 0 {
			prometheusCRConfig["scrape_interval"] = taSpec.PrometheusCR.ScrapeInterval.Duration
		}

		prometheusCRConfig["service_monitor_selector"] = taSpec.PrometheusCR.ServiceMonitorSelector

		prometheusCRConfig["pod_monitor_selector"] = taSpec.PrometheusCR.PodMonitorSelector

		taConfig["prometheus_cr"] = prometheusCRConfig
	}

	taConfigYAML, err := yaml.Marshal(taConfig)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: instance.Annotations,
		},
		Data: map[string]string{
			targetAllocatorFilename: string(taConfigYAML),
		},
	}, nil
}
