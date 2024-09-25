// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"os"
	"testing"

	"gopkg.in/yaml.v2"

	colfeaturegate "go.opentelemetry.io/collector/featuregate"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"

	"github.com/stretchr/testify/assert"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected cwagent config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLables["app.kubernetes.io/name"] = "test"
		expectedLables["app.kubernetes.io/version"] = "0.0.0"

		expectedData := map[string]string{
			"cwagentconfig.json": `{"logs":{"metrics_collected":{"application_signals":{},"kubernetes":{"enhanced_container_insights":true}}},"traces":{"traces_collected":{"application_signals":{}}}}`,
		}

		param := deploymentParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}

func TestDesiredPrometheusConfigMap(t *testing.T) {
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
	}

	configYAML, err := os.ReadFile("testdata/prometheus_test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	promCfg := v1alpha1.PrometheusConfig{}
	err = yaml.Unmarshal(configYAML, &promCfg)
	if err != nil {
		fmt.Printf("failed to unmarshal config: %v", err)
	}

	httpConfigYAML, err := os.ReadFile("testdata/http_sd_config_servicemonitor_test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	httpPromCfg := v1alpha1.PrometheusConfig{}
	err = yaml.Unmarshal(httpConfigYAML, &httpPromCfg)
	if err != nil {
		fmt.Printf("failed to unmarshal config: %v", err)
	}

	httpTAConfigYAML, err := os.ReadFile("testdata/http_sd_config_servicemonitor_test_ta_set.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	httpTAPromCfg := v1alpha1.PrometheusConfig{}
	err = yaml.Unmarshal(httpTAConfigYAML, &httpTAPromCfg)
	if err != nil {
		fmt.Printf("failed to unmarshal config: %v", err)
	}

	t.Run("should return expected prometheus config map", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test-prometheus-config"

		expectedData := map[string]string{
			"prometheus.yaml": `config:
  scrape_configs:
  - job_name: cloudwatch-agent
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
`,
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
				},
			},
		}
		actual, err := PrometheusConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-prometheus-config", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})

	t.Run("should return expected prometheus config map with http_sd_config if rewrite flag disabled", func(t *testing.T) {
		err := colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), false)
		assert.NoError(t, err)
		t.Cleanup(func() {
			_ = colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), true)
		})
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test-prometheus-config"

		expectedData := map[string]string{
			"prometheus.yaml": `config:
  scrape_configs:
  - http_sd_configs:
    - url: http://test-target-allocator:80/jobs/cloudwatch-agent/targets
    job_name: cloudwatch-agent
    scrape_interval: 10s
`,
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
				},
			},
		}
		param.OtelCol.Spec.TargetAllocator.Enabled = true
		actual, err := PrometheusConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-prometheus-config", actual.GetName())
		assert.Equal(t, expectedLabels, actual.GetLabels())
		assert.Equal(t, expectedData, actual.Data)

	})

	t.Run("should return expected escaped prometheus config map with http_sd_config if rewrite flag disabled", func(t *testing.T) {
		err := colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), false)
		assert.NoError(t, err)
		t.Cleanup(func() {
			_ = colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), true)
		})

		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test-prometheus-config"

		expectedData := map[string]string{
			"prometheus.yaml": `config:
  scrape_configs:
  - http_sd_configs:
    - url: http://test-target-allocator:80/jobs/serviceMonitor%2Ftest%2Ftest%2F0/targets
    job_name: serviceMonitor/test/test/0
target_allocator:
  endpoint: http://test-target-allocator:80
  http_sd_config:
    refresh_interval: 60s
  interval: 30s
`,
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
					Prometheus: httpTAPromCfg,
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Enabled: true,
						Image:   "test/test-img",
					},
				},
			},
		}
		assert.NoError(t, err)
		param.OtelCol.Spec.TargetAllocator.Enabled = true
		actual, err := PrometheusConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-prometheus-config", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})

	t.Run("should return expected escaped prometheus config map with target_allocator config block", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test-prometheus-config"

		expectedData := map[string]string{
			"prometheus.yaml": `config: {}
target_allocator:
  endpoint: http://test-target-allocator:80
  interval: 30s
`,
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
					Prometheus: httpPromCfg,
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Enabled: true,
						Image:   "test/test-img",
					},
				},
			},
		}
		assert.NoError(t, err)
		param.OtelCol.Spec.TargetAllocator.Enabled = true
		actual, err := PrometheusConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-prometheus-config", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}
