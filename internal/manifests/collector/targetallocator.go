// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator/adapters"
)

// TargetAllocator builds the TargetAllocator CR for the given instance.
func TargetAllocator(params manifests.Params) (*v1alpha1.TargetAllocator, error) {

	taSpec := params.OtelCol.Spec.TargetAllocator
	if !taSpec.Enabled {
		return nil, nil
	}

	configStr, err := params.OtelCol.Spec.Config.Yaml()
	if err != nil {
		return nil, err
	}
	scrapeConfigs, err := getScrapeConfigs(configStr)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.TargetAllocator{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.OtelCol.Name,
			Namespace:   params.OtelCol.Namespace,
			Annotations: params.OtelCol.Annotations,
			Labels:      params.OtelCol.Labels,
		},
		Spec: v1alpha1.TargetAllocatorSpec{
			AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
				Replicas:                  taSpec.Replicas,
				NodeSelector:              taSpec.NodeSelector,
				Resources:                 taSpec.Resources,
				ServiceAccount:            taSpec.ServiceAccount,
				Image:                     taSpec.Image,
				Affinity:                  taSpec.Affinity,
				SecurityContext:           taSpec.SecurityContext,
				PodSecurityContext:        taSpec.PodSecurityContext,
				TopologySpreadConstraints: taSpec.TopologySpreadConstraints,
				Tolerations:               taSpec.Tolerations,
				Env:                       taSpec.Env,
				PodAnnotations:            params.OtelCol.Spec.PodAnnotations,
				PodDisruptionBudget:       taSpec.PodDisruptionBudget,
			},
			AllocationStrategy: taSpec.AllocationStrategy,
			FilterStrategy:     taSpec.FilterStrategy,
			ScrapeConfigs:      scrapeConfigs,
			PrometheusCR:       taSpec.PrometheusCR,
			Observability:      taSpec.Observability,
		},
	}, nil
}

func getScrapeConfigs(otelcolConfig string) ([]v1alpha1.AnyConfig, error) {
	// Collector supports environment variable substitution, but the TA does not.
	// TA Scrape Configs should have a single "$", as it does not support env var substitution
	prometheusReceiverConfig, err := adapters.UnescapeDollarSignsInPromConfig(otelcolConfig)
	if err != nil {
		return nil, err
	}

	scrapeConfigs, err := adapters.GetScrapeConfigsFromPromConfig(prometheusReceiverConfig)
	if err != nil {
		return nil, err
	}

	v1alpha1scrapeConfigs := make([]v1alpha1.AnyConfig, len(scrapeConfigs))

	for i, config := range scrapeConfigs {
		v1alpha1scrapeConfigs[i] = v1alpha1.AnyConfig{Object: config}
	}

	return v1alpha1scrapeConfigs, nil
}
