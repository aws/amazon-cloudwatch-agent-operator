// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestEnabledAcceleratedComputeByAgentConfig(t *testing.T) {
	ctx := context.Background()
	logger := logf.Log.WithName("unit-tests")
	dcgmOtelConfig := `
receivers:
  prometheus/dcgm:
    config:
      scrape_configs:
        - job_name: dcgm-exporter
          static_configs:
            - targets: [dcgm-exporter-service:9400]
service:
  pipelines:
    metrics/dcgm:
      receivers: [prometheus/dcgm]
`
	testCases := []struct {
		name       string
		config     string
		otelConfig string
		expected   bool
	}{
		{
			name:     "disabledEnhancedContainerInsights",
			config:   `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":false}}}}`,
			expected: false,
		},
		{
			name:     "missingAcceleratedComputeMetric",
			config:   `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true}}}}`,
			expected: true,
		},
		{
			name:     "disabledAcceleratedComputeMetric",
			config:   `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true, "accelerated_compute_metrics":false}}}}`,
			expected: false,
		},
		{
			name:     "enabledAcceleratedComputeMetric",
			config:   `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true, "accelerated_compute_metrics":true}}}}`,
			expected: true,
		},
		{
			name:     "mixedCaseWithDisabledEnhanced",
			config:   `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":false, "accelerated_compute_metrics":true}}}}`,
			expected: false,
		},
		{
			name:     "missingKubernetesBlock",
			config:   `{"logs":{"metrics_collected":{}}}`,
			expected: false,
		},
		{
			name:     "malformed",
			config:   `"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":false, "accelerated_compute_metrics":true}}}}`,
			expected: false,
		},
		{
			name:       "otelPipelineScrapesService",
			config:     `{}`,
			otelConfig: dcgmOtelConfig,
			expected:   true,
		},
		{
			name:       "otelPipelineScrapesOtherService",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "dcgm-exporter-service", "neuron-monitor-service"),
			expected:   false,
		},
		{
			name:       "otelReceiverNotInPipeline",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "receivers: [prometheus/dcgm]", "receivers: [otlp]"),
			expected:   false,
		},
		{
			name:       "otelMalformed",
			config:     `{}`,
			otelConfig: "service: [",
			expected:   false,
		},
	}

	for _, tc := range testCases {
		getAmazonCloudWatchAgentResource = func(ctx context.Context, c client.Client) v1alpha1.AmazonCloudWatchAgent {
			return v1alpha1.AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					Config:     tc.config,
					OtelConfig: tc.otelConfig,
				},
			}
		}
		actual := enabledAcceleratedComputeByAgentConfig(ctx, nil, logger, "dcgm-exporter-service")
		assert.Equal(t, tc.expected, actual)
	}
}
