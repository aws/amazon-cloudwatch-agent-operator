// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

const (
	targetAllocatorFilename = "targetallocator.yaml"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := naming.TAConfigMap(params.OtelCol.Name)
	version := strings.Split(params.OtelCol.Spec.TargetAllocator.Image, ":")
	labels := Labels(params.OtelCol, name)
	if len(version) > 1 {
		labels["app.kubernetes.io/version"] = version[len(version)-1]
	} else {
		labels["app.kubernetes.io/version"] = "latest"
	}

	promConfigYaml, err := params.OtelCol.Spec.Prometheus.Yaml()
	if err != nil {
		return &corev1.ConfigMap{}, fmt.Errorf("%s could not convert json to yaml", err)
	}

	prometheusConfig, err := adapters.GetPromConfig(promConfigYaml)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	taConfig := make(map[interface{}]interface{})
	prometheusCRConfig := make(map[interface{}]interface{})
	taConfig["label_selector"] = manifestutils.SelectorLabels(params.OtelCol.ObjectMeta, collector.ComponentAmazonCloudWatchAgent)
	// We only take the "config" from the returned object, if it's present
	if prometheusConfig, ok := prometheusConfig["config"]; ok {
		taConfig["config"] = prometheusConfig
	}

	// Use the strategy from the CR if set, defaulting to consistent-hashing to
	// preserve prior behavior. When per-node is selected, configure a
	// consistent-hashing fallback so targets without a resolvable node (e.g.
	// non-pod ServiceMonitor endpoints) are still allocated rather than dropped.
	allocationStrategy := params.OtelCol.Spec.TargetAllocator.AllocationStrategy
	if allocationStrategy == "" {
		allocationStrategy = v1alpha1.AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing
	}
	taConfig["allocation_strategy"] = allocationStrategy
	if allocationStrategy == v1alpha1.AmazonCloudWatchAgentTargetAllocatorAllocationStrategyPerNode {
		taConfig["allocation_fallback_strategy"] = v1alpha1.AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing
	}

	if len(params.OtelCol.Spec.TargetAllocator.FilterStrategy) > 0 {
		taConfig["filter_strategy"] = params.OtelCol.Spec.TargetAllocator.FilterStrategy
	}

	if params.OtelCol.Spec.TargetAllocator.PrometheusCR.ScrapeInterval.Size() > 0 {
		prometheusCRConfig["scrape_interval"] = params.OtelCol.Spec.TargetAllocator.PrometheusCR.ScrapeInterval.Duration
	}

	if params.OtelCol.Spec.TargetAllocator.PrometheusCR.ServiceMonitorSelector != nil {
		taConfig["service_monitor_selector"] = &params.OtelCol.Spec.TargetAllocator.PrometheusCR.ServiceMonitorSelector
	}

	if params.OtelCol.Spec.TargetAllocator.PrometheusCR.PodMonitorSelector != nil {
		taConfig["pod_monitor_selector"] = &params.OtelCol.Spec.TargetAllocator.PrometheusCR.PodMonitorSelector
	}

	if len(prometheusCRConfig) > 0 {
		taConfig["prometheus_cr"] = prometheusCRConfig
	}

	taConfigYAML, err := yaml.Marshal(taConfig)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OtelCol.Namespace,
			Labels:      labels,
			Annotations: params.OtelCol.Annotations,
		},
		Data: map[string]string{
			targetAllocatorFilename: string(taConfigYAML),
		},
	}, nil
}
