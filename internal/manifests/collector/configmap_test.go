// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"os"
	"testing"

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
		"app.kubernetes.io/version":    "0.47.0",
	}

	configYAML, err := os.ReadFile("testdata/prometheus_test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}

	t.Run("should return expected prometheus config map", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test-prometheus-config"
		expectedLabels["app.kubernetes.io/version"] = "0.0.0"

		expectedData := map[string]string{
			"prometheus.yaml": `scrape_configs:
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
					Prometheus: string(configYAML),
				},
			},
		}
		actual, err := PrometheusConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-prometheus-config", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)
	})
}
