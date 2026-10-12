// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestEnabledAcceleratedComputeByAgentConfig(t *testing.T) {
	ctx := context.Background()
	logger := logf.Log.WithName("unit-tests")
	helmOtelConfig, err := os.ReadFile("testdata/helm_otel_container_insights.yaml")
	assert.NoError(t, err)
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
		name        string
		config      string
		otelConfig  string
		service     string
		otherAgents []v1alpha1.AmazonCloudWatchAgent
		expected    bool
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
		{
			name:       "otelTargetLongerServiceName",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "dcgm-exporter-service:9400", "dcgm-exporter-service-foo:9400"),
			expected:   false,
		},
		{
			name:       "otelTargetServiceDNSName",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "dcgm-exporter-service:9400", "dcgm-exporter-service.amazon-cloudwatch.svc:9400"),
			expected:   true,
		},
		{
			name:       "otelServiceOnlyInRelabelRule",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "- targets: [dcgm-exporter-service:9400]", "- targets: [other:9400]\n          relabel_configs:\n            - action: drop\n              regex: dcgm-exporter-service"),
			expected:   false,
		},
		{
			name:       "otelPipelineScrapesNeuronService",
			config:     `{}`,
			otelConfig: strings.ReplaceAll(dcgmOtelConfig, "dcgm-exporter-service:9400", "neuron-monitor-service:8000"),
			service:    "neuron-monitor-service",
			expected:   true,
		},
		{
			name:       "otelPipelineScrapesServiceWithLegacyOptOut",
			config:     `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true, "accelerated_compute_metrics":false}}}}`,
			otelConfig: dcgmOtelConfig,
			expected:   false,
		},
		{
			name:       "helmOtelContainerInsightsDcgm",
			otelConfig: string(helmOtelConfig),
			expected:   true,
		},
		{
			name:       "helmOtelContainerInsightsNeuron",
			otelConfig: string(helmOtelConfig),
			service:    "neuron-monitor-service",
			expected:   true,
		},
		{
			name:        "legacyOptOutWithAnotherAgentEnablingEnhancedContainerInsights",
			config:      `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true, "accelerated_compute_metrics":false}}}}`,
			otelConfig:  dcgmOtelConfig,
			otherAgents: []v1alpha1.AmazonCloudWatchAgent{{Spec: v1alpha1.AmazonCloudWatchAgentSpec{Config: `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true}}}}`}}},
			expected:    false,
		},
		{
			name:   "windowsAgentEnablingEnhancedContainerInsights",
			config: `{}`,
			otherAgents: []v1alpha1.AmazonCloudWatchAgent{{Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				NodeSelector: map[string]string{"kubernetes.io/os": "windows"},
				Config:       `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true}}}}`,
			}}},
			expected: false,
		},
		{
			name:        "otelPipelineOnAnotherAgent",
			config:      `{}`,
			otherAgents: []v1alpha1.AmazonCloudWatchAgent{{Spec: v1alpha1.AmazonCloudWatchAgentSpec{OtelConfig: dcgmOtelConfig}}},
			expected:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listAmazonCloudWatchAgents = func(ctx context.Context, c client.Client, namespace string) ([]v1alpha1.AmazonCloudWatchAgent, error) {
				return append([]v1alpha1.AmazonCloudWatchAgent{{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.AmazonCloudWatchAgentSpec{
						Config:     tc.config,
						OtelConfig: tc.otelConfig,
					},
				}}, tc.otherAgents...), nil
			}
			service := tc.service
			if service == "" {
				service = "dcgm-exporter-service"
			}
			actual, err := enabledAcceleratedComputeByAgentConfig(ctx, nil, logger, "amazon-cloudwatch", service)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestEnabledAcceleratedComputeByAgentConfigListError(t *testing.T) {
	listAmazonCloudWatchAgents = func(ctx context.Context, c client.Client, namespace string) ([]v1alpha1.AmazonCloudWatchAgent, error) {
		return nil, errors.New("cache not synced")
	}
	enabled, err := enabledAcceleratedComputeByAgentConfig(context.Background(), nil, logf.Log.WithName("unit-tests"), "amazon-cloudwatch", "dcgm-exporter-service")
	assert.Error(t, err)
	assert.False(t, enabled)
}

func TestDcgmExportersUsingConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))
	selector := func(name string) *v1alpha1.MetricsConfigSource {
		return &v1alpha1.MetricsConfigSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "gpu.csv"}}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1alpha1.DcgmExporter{ObjectMeta: metav1.ObjectMeta{Name: "uses", Namespace: "amazon-cloudwatch"}, Spec: v1alpha1.DcgmExporterSpec{MetricsConfigFrom: selector("my-metrics")}},
		&v1alpha1.DcgmExporter{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "amazon-cloudwatch"}, Spec: v1alpha1.DcgmExporterSpec{MetricsConfigFrom: selector("other-metrics")}},
		&v1alpha1.DcgmExporter{ObjectMeta: metav1.ObjectMeta{Name: "inline", Namespace: "amazon-cloudwatch"}},
		&v1alpha1.DcgmExporter{ObjectMeta: metav1.ObjectMeta{Name: "uses", Namespace: "other"}, Spec: v1alpha1.DcgmExporterSpec{MetricsConfigFrom: selector("my-metrics")}},
	).Build()
	r := &DcgmExporterReconciler{Client: c, log: logf.Log}

	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-metrics", Namespace: "amazon-cloudwatch"}}
	assert.Equal(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "amazon-cloudwatch", Name: "uses"}}},
		r.exportersUsingConfigMap(context.Background(), configMap))
}
