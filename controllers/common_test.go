// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
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

func TestDcgmExporterMissingMetricsConfigMap(t *testing.T) {
	orig := listAmazonCloudWatchAgents
	t.Cleanup(func() { listAmazonCloudWatchAgents = orig })
	scheme := runtime.NewScheme()
	assert.NoError(t, clientgoscheme.AddToScheme(scheme))
	assert.NoError(t, v1alpha1.AddToScheme(scheme))
	enhancedContainerInsights := `{"logs":{"metrics_collected":{"kubernetes":{"enhanced_container_insights":true}}}}`
	for _, tc := range []struct {
		name         string
		optional     *bool
		customerData map[string]string
		applied      bool
		agentConfig  string
		getError     bool
		expected     string
		warning      bool
	}{
		{name: "requiredPresent", customerData: map[string]string{"gpu.csv": "custom"}, applied: true, agentConfig: enhancedContainerInsights, expected: "custom"},
		{name: "optionalPresent", optional: ptr.To(true), customerData: map[string]string{"gpu.csv": "custom"}, applied: true, agentConfig: enhancedContainerInsights, expected: "custom"},
		{name: "requiredKeepsAppliedMetrics", applied: true, agentConfig: enhancedContainerInsights, expected: "applied", warning: true},
		{name: "requiredOnFirstInstall", agentConfig: enhancedContainerInsights, expected: "defaults", warning: true},
		{name: "requiredAppliedConfigMapGetError", applied: true, getError: true, agentConfig: enhancedContainerInsights, expected: "applied"},
		{name: "optionalNotFound", optional: ptr.To(true), applied: true, agentConfig: enhancedContainerInsights, expected: "defaults"},
		{name: "optionalEmptyKey", optional: ptr.To(true), customerData: map[string]string{"gpu.csv": " "}, applied: true, agentConfig: enhancedContainerInsights, expected: "defaults", warning: true},
		{name: "requiredWithAcceleratedComputeDisabled", applied: true, agentConfig: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listAmazonCloudWatchAgents = func(ctx context.Context, c client.Client, namespace string) ([]v1alpha1.AmazonCloudWatchAgent, error) {
				return []v1alpha1.AmazonCloudWatchAgent{{Spec: v1alpha1.AmazonCloudWatchAgentSpec{Config: tc.agentConfig}}}, nil
			}
			exporter := &v1alpha1.DcgmExporter{
				ObjectMeta: metav1.ObjectMeta{Name: "dcgm-exporter", Namespace: "amazon-cloudwatch"},
				Spec: v1alpha1.DcgmExporterSpec{
					MetricsConfig: "defaults",
					MetricsConfigFrom: &v1alpha1.MetricsConfigSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "customer"}, Key: "gpu.csv", Optional: tc.optional,
					}},
				},
			}
			objects := []client.Object{exporter}
			if tc.customerData != nil {
				objects = append(objects, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "customer", Namespace: "amazon-cloudwatch"}, Data: tc.customerData})
			}
			if tc.applied {
				objects = append(objects, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "dcgm-exporter-config-map", Namespace: "amazon-cloudwatch"},
					Data:       map[string]string{"dcp-metrics-included.csv": "applied"},
				})
			}
			builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(exporter)
			if tc.getError {
				failed := false
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if key.Name == "dcgm-exporter-config-map" && !failed {
						failed = true
						return errors.New("cache not synced")
					}
					return c.Get(ctx, key, obj, opts...)
				}})
			}
			c := builder.Build()
			recorder := record.NewFakeRecorder(10)
			r := NewDcgmExporterReconciler(Params{Client: c, Log: logf.Log, Scheme: scheme, Config: config.New(), Recorder: recorder})

			_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "amazon-cloudwatch", Name: "dcgm-exporter"}})
			if tc.getError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			warned := false
			for len(recorder.Events) > 0 {
				if strings.Contains(<-recorder.Events, "MetricsConfigMapNotFound") {
					warned = true
				}
			}
			assert.Equal(t, tc.warning, warned)

			managed := &corev1.ConfigMap{}
			err = c.Get(context.Background(), types.NamespacedName{Namespace: "amazon-cloudwatch", Name: "dcgm-exporter-config-map"}, managed)
			daemonSet := &appsv1.DaemonSet{}
			daemonSetErr := c.Get(context.Background(), types.NamespacedName{Namespace: "amazon-cloudwatch", Name: "dcgm-exporter"}, daemonSet)
			if tc.expected == "" {
				assert.True(t, apierrors.IsNotFound(err))
				assert.True(t, apierrors.IsNotFound(daemonSetErr))
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, managed.Data["dcp-metrics-included.csv"])
			assert.NoError(t, daemonSetErr)
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(tc.expected))), daemonSet.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"])
		})
	}
}
