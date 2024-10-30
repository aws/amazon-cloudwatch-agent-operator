// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/prometheus/prometheus/discovery/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
)

func TestPrometheusParser(t *testing.T) {
	httpConfigYAML, err := os.ReadFile("testdata/http_sd_config_test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	promCfg := v1alpha1.PrometheusConfig{}
	err = yaml.Unmarshal(httpConfigYAML, &promCfg)
	if err != nil {
		fmt.Printf("failed to unmarshal config: %v", err)
	}
	param := manifests.Params{
		OtelCol: v1alpha1.AmazonCloudWatchAgent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       instanceUID,
			},
			Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				Image:      "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:0.0.0",
				Prometheus: promCfg,
				TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
					Enabled: true,
					Image:   "test/test-img",
				},
			},
		},
	}
	assert.NoError(t, err)

	t.Run("should update config with http_sd_config", func(t *testing.T) {
		err := colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), false)
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), true)
		})
		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		// prepare
		var cfg Config
		promCfgMap, err := adapters.ConfigFromString(actualConfig)
		assert.NoError(t, err)

		promCfg, err := yaml.Marshal(promCfgMap)
		assert.NoError(t, err)

		err = yaml.UnmarshalStrict(promCfg, &cfg)
		assert.NoError(t, err)

		// test
		expectedMap := map[string]bool{
			"prometheus": false,
			"service-x":  false,
		}
		for _, scrapeConfig := range cfg.PromConfig.ScrapeConfigs {
			assert.Len(t, scrapeConfig.ServiceDiscoveryConfigs, 1)
			assert.Equal(t, scrapeConfig.ServiceDiscoveryConfigs[0].Name(), "http")
			assert.Equal(t, scrapeConfig.ServiceDiscoveryConfigs[0].(*http.SDConfig).URL, fmt.Sprintf("https://%s-target-allocator-service:80/jobs/", param.OtelCol.Name)+scrapeConfig.JobName+"/targets")
			expectedMap[scrapeConfig.JobName] = true
		}
		for k := range expectedMap {
			assert.True(t, expectedMap[k], k)
		}
		assert.True(t, cfg.TargetAllocConfig == nil)
	})

	t.Run("should update config with targetAllocator block if block not present", func(t *testing.T) {
		// Set up the test scenario
		param.OtelCol.Spec.TargetAllocator.Enabled = true
		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		// Verify the expected changes in the config
		promCfgMap, err := adapters.ConfigFromString(actualConfig)
		assert.NoError(t, err)

		prometheusConfig := promCfgMap["config"].(map[interface{}]interface{})

		assert.NotContains(t, prometheusConfig, "scrape_configs")

		expectedTAConfig := map[interface{}]interface{}{
			"endpoint": fmt.Sprintf("https://%s-target-allocator-service:80", param.OtelCol.Name),
			"interval": "30s",
		}
		assert.Equal(t, expectedTAConfig, promCfgMap["target_allocator"])
		assert.NoError(t, err)
	})

	t.Run("should update config with targetAllocator block if block already present", func(t *testing.T) {
		// Set up the test scenario
		httpTAConfigYAML, err := os.ReadFile("testdata/http_sd_config_ta_test.yaml")
		if err != nil {
			fmt.Printf("Error getting yaml file: %v", err)
		}
		promCfg := v1alpha1.PrometheusConfig{}
		err = yaml.Unmarshal(httpTAConfigYAML, &promCfg)
		if err != nil {
			fmt.Printf("failed to unmarshal config: %v", err)
		}
		paramTa := manifests.Params{
			OtelCol: v1alpha1.AmazonCloudWatchAgent{
				TypeMeta: metav1.TypeMeta{
					Kind:       "cloudwatch.aws.amazon.com",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					UID:       instanceUID,
				},
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					Image:      "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:0.0.0",
					Prometheus: promCfg,
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Enabled: true,
						Image:   "test/test-img",
					},
				},
			},
		}
		require.NoError(t, err)
		paramTa.OtelCol.Spec.TargetAllocator.Enabled = true

		actualConfig, err := ReplacePrometheusConfig(paramTa.OtelCol)
		assert.NoError(t, err)

		// Verify the expected changes in the config
		promCfgMap, err := adapters.ConfigFromString(actualConfig)
		assert.NoError(t, err)

		prometheusConfig := promCfgMap["config"].(map[interface{}]interface{})

		assert.NotContains(t, prometheusConfig, "scrape_configs")

		expectedTAConfig := map[interface{}]interface{}{
			"endpoint": fmt.Sprintf("https://%s-target-allocator-service:80", param.OtelCol.Name),
			"interval": "30s",
		}
		assert.Equal(t, expectedTAConfig, promCfgMap["target_allocator"])
		assert.NoError(t, err)
	})

	t.Run("should not update config with http_sd_config", func(t *testing.T) {
		param.OtelCol.Spec.TargetAllocator.Enabled = false
		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		// prepare
		var cfg Config
		promCfgMap, err := adapters.ConfigFromString(actualConfig)
		assert.NoError(t, err)

		promCfg, err := yaml.Marshal(promCfgMap)
		assert.NoError(t, err)

		err = yaml.UnmarshalStrict(promCfg, &cfg)
		assert.NoError(t, err)

		// test
		expectedMap := map[string]bool{
			"prometheus": false,
			"service-x":  false,
		}
		for _, scrapeConfig := range cfg.PromConfig.ScrapeConfigs {
			assert.Len(t, scrapeConfig.ServiceDiscoveryConfigs, 2)
			assert.Equal(t, scrapeConfig.ServiceDiscoveryConfigs[0].Name(), "file")
			assert.Equal(t, scrapeConfig.ServiceDiscoveryConfigs[1].Name(), "static")
			expectedMap[scrapeConfig.JobName] = true
		}
		for k := range expectedMap {
			assert.True(t, expectedMap[k], k)
		}
		assert.True(t, cfg.TargetAllocConfig == nil)
	})

}

func TestReplacePrometheusConfig(t *testing.T) {
	relabelConfigYAML, err := os.ReadFile("testdata/relabel_config_original.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	promCfg := v1alpha1.PrometheusConfig{}
	err = yaml.Unmarshal(relabelConfigYAML, &promCfg)
	if err != nil {
		fmt.Printf("failed to unmarshal config: %v", err)
	}
	param := manifests.Params{
		OtelCol: v1alpha1.AmazonCloudWatchAgent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       instanceUID,
			},
			Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				Image:      "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:0.0.0",
				Prometheus: promCfg,
				TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
					Enabled: true,
					Image:   "test/test-img",
				},
			},
		},
	}
	assert.NoError(t, err)

	t.Run("should not modify config when TargetAllocator is disabled", func(t *testing.T) {
		param.OtelCol.Spec.TargetAllocator.Enabled = false

		expectedConfig := `config:
  global:
    evaluation_interval: 1m
    scrape_interval: 1m
    scrape_timeout: 10s
  scrape_configs:
  - honor_labels: true
    job_name: service-x
    metric_relabel_configs:
    - action: keep
      regex: (.*)
      separator: ;
      source_labels:
      - label1
    - action: labelmap
      regex: (.*)
      separator: ;
      source_labels:
      - label4
    metrics_path: /metrics
    relabel_configs:
    - action: keep
      regex: (.*)
      source_labels:
      - label1
    - action: replace
      regex: (.*)
      replacement: $1_$2
      separator: ;
      source_labels:
      - label2
      target_label: label3
    - action: labelmap
      regex: (.*)
      separator: ;
      source_labels:
      - label4
    - action: labeldrop
      regex: foo_.*
    scheme: http
    scrape_interval: 1m
    scrape_timeout: 10s
`

		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		assert.Equal(t, expectedConfig, actualConfig)
	})

	t.Run("should rewrite scrape configs with SD config when TargetAllocator is enabled and feature flag is not set", func(t *testing.T) {
		err := colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), false)
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), true)
		})

		param.OtelCol.Spec.TargetAllocator.Enabled = true

		expectedConfig := `config:
  global:
    evaluation_interval: 1m
    scrape_interval: 1m
    scrape_timeout: 10s
  scrape_configs:
  - honor_labels: true
    http_sd_configs:
    - url: https://test-target-allocator-service:80/jobs/service-x/targets
    job_name: service-x
    metric_relabel_configs:
    - action: keep
      regex: (.*)
      separator: ;
      source_labels:
      - label1
    - action: labelmap
      regex: (.*)
      separator: ;
      source_labels:
      - label4
    metrics_path: /metrics
    relabel_configs:
    - action: keep
      regex: (.*)
      source_labels:
      - label1
    - action: replace
      regex: (.*)
      replacement: $1_$2
      separator: ;
      source_labels:
      - label2
      target_label: label3
    - action: labelmap
      regex: (.*)
      separator: ;
      source_labels:
      - label4
    - action: labeldrop
      regex: foo_.*
    scheme: http
    scrape_interval: 1m
    scrape_timeout: 10s
`

		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		assert.Equal(t, expectedConfig, actualConfig)
	})

	t.Run("should remove scrape configs if TargetAllocator is enabled and feature flag is set", func(t *testing.T) {
		param.OtelCol.Spec.TargetAllocator.Enabled = true

		expectedConfig := `config:
  global:
    evaluation_interval: 1m
    scrape_interval: 1m
    scrape_timeout: 10s
target_allocator:
  endpoint: https://test-target-allocator-service:80
  interval: 30s
`

		actualConfig, err := ReplacePrometheusConfig(param.OtelCol)
		assert.NoError(t, err)

		assert.Equal(t, expectedConfig, actualConfig)
	})
}

// TestReplaceConfig tests the ReplaceConfig function when logs::metrics_collected::prometheus is present
func TestReplaceConfigLogsPrometheus(t *testing.T) {
	jsonConfig := `{
		"logs": {
			"metrics_collected": {
				"prometheus": {
					"prometheus_config_path": "/custom/path/logs_prometheus.yaml"
				}
			}
		}
	}`

	agent := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: jsonConfig,
			Prometheus: v1alpha1.PrometheusConfig{
				Config: &v1alpha1.AnyConfig{},
			},
		},
		Status: v1alpha1.AmazonCloudWatchAgentStatus{},
	}

	result, err := ReplaceConfig(agent)
	assert.NoError(t, err, "Expected no error while replacing config")

	expected := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"prometheus_config_path": "/custom/path/logs_prometheus.yaml",
				},
			},
		},
	}

	expectedJSON, err := json.Marshal(expected)
	assert.NoError(t, err, "Expected no error while marshaling expected result")

	assert.JSONEq(t, string(expectedJSON), result, "The resulting JSON should match the expected JSON")
}

// TestReplaceConfigMetricsPrometheus tests the ReplaceConfig function when metrics::metrics_collected::prometheus is present
func TestReplaceConfigMetricsPrometheus(t *testing.T) {
	jsonConfig := `{
		"metrics": {
			"metrics_collected": {
				"prometheus": {
					"prometheus_config_path": "/custom/path/metrics_prometheus.yaml"
				}
			}
		}
	}`

	agent := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: jsonConfig,
			Prometheus: v1alpha1.PrometheusConfig{
				Config: &v1alpha1.AnyConfig{},
			},
		},
		Status: v1alpha1.AmazonCloudWatchAgentStatus{},
	}

	result, err := ReplaceConfig(agent)
	assert.NoError(t, err, "Expected no error while replacing config")

	expected := map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"prometheus_config_path": "/custom/path/metrics_prometheus.yaml",
				},
			},
		},
	}

	// Marshal expected result to JSON
	expectedJSON, err := json.Marshal(expected)
	assert.NoError(t, err, "Expected no error while marshaling expected result")

	// Assert the output of ReplaceConfig matches the expected JSON
	assert.JSONEq(t, string(expectedJSON), result, "The resulting JSON should match the expected JSON")
}

// TestReplaceConfigWithDefaultPath tests the ReplaceConfig function when neither logs nor metrics prometheus path is set
func TestReplaceConfigWithDefaultPath(t *testing.T) {
	jsonConfig := `{
		"logs": {
			"metrics_collected": {
				"prometheus": {}
			}
		},
		"metrics": {
			"metrics_collected": {
				"prometheus": {}
			}
		}
	}`

	agent := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: jsonConfig,
			Prometheus: v1alpha1.PrometheusConfig{
				Config: &v1alpha1.AnyConfig{},
			},
		},
		Status: v1alpha1.AmazonCloudWatchAgentStatus{},
	}

	result, err := ReplaceConfig(agent)
	assert.NoError(t, err, "Expected no error while replacing config")

	expected := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"prometheus_config_path": "/etc/prometheusconfig/prometheus.yaml",
				},
			},
		},
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"prometheus_config_path": "/etc/prometheusconfig/prometheus.yaml",
				},
			},
		},
	}

	expectedJSON, err := json.Marshal(expected)
	assert.NoError(t, err, "Expected no error while marshaling expected result")

	assert.JSONEq(t, string(expectedJSON), result, "The resulting JSON should match the expected JSON")
}

func TestReplaceConfigWithDefaultPathButNoConfigmap(t *testing.T) {
	jsonConfig := `{
		"logs": {
			"metrics_collected": {
				"prometheus": {}
			}
		},
		"metrics": {
			"metrics_collected": {
				"prometheus": {}
			}
		}
	}`

	agent := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: jsonConfig,
		},
		Status: v1alpha1.AmazonCloudWatchAgentStatus{},
	}

	result, err := ReplaceConfig(agent)
	assert.NoError(t, err, "Expected no error while replacing config")

	expected := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{},
			},
		},
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"prometheus": map[string]interface{}{},
			},
		},
	}

	expectedJSON, err := json.Marshal(expected)
	assert.NoError(t, err, "Expected no error while marshaling expected result")

	assert.JSONEq(t, string(expectedJSON), result, "The resulting JSON should match the expected JSON")
}

// TestReplaceConfigNoPrometheusSection tests the ReplaceConfig function when neither `logs` nor `metrics` sections have `prometheus` defined
func TestReplaceConfigNoPrometheusSection(t *testing.T) {
	jsonConfig := `{
		"logs": {
			"metrics_collected": {}
		},
		"metrics": {
			"metrics_collected": {}
		}
	}`

	agent := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: jsonConfig,
			Prometheus: v1alpha1.PrometheusConfig{
				Config: &v1alpha1.AnyConfig{},
			},
		},
		Status: v1alpha1.AmazonCloudWatchAgentStatus{},
	}

	result, err := ReplaceConfig(agent)
	assert.NoError(t, err, "Expected no error while replacing config")

	expected := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{},
		},
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{},
		},
	}

	expectedJSON, err := json.Marshal(expected)
	assert.NoError(t, err, "Expected no error while marshaling expected result")

	assert.JSONEq(t, string(expectedJSON), result, "The resulting JSON should match the expected JSON")
}
