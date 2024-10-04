// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected cwagent config map", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test"
		expectedLabels["app.kubernetes.io/version"] = "0.0.0"

		expectedData := map[string]string{
			"cwagentconfig.json": `{"logs":{"metrics_collected":{"application_signals":{},"kubernetes":{"enhanced_container_insights":true}}},"traces":{"traces_collected":{"application_signals":{}}}}`,
		}

		param := deploymentParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}

func TestDesiredConfigMapWithOtelConfigSupplied(t *testing.T) {
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected cwagent config map", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLabels["app.kubernetes.io/name"] = "test"
		expectedLabels["app.kubernetes.io/version"] = "0.0.0"

		expectedData := map[string]string{
			"cwagentconfig.json": `{"logs":{"metrics_collected":{"application_signals":{},"kubernetes":{"enhanced_container_insights":true}}},"traces":{"traces_collected":{"application_signals":{}}}}`,
			"cwagentotelconfig.yaml": `receivers:
  jaeger:
    protocols:
      grpc:
  prometheus:
    config:
      scrape_configs:
      - job_name: otel-collector
        scrape_interval: 10s
        static_configs:
          - targets: [ '0.0.0.0:8888', '0.0.0.0:9999' ]

exporters:
  debug:

service:
  pipelines:
    metrics:
      receivers: [prometheus, jaeger]
      exporters: [debug]`,
		}

		param := otelConfigParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData["cwagentconfig.json"], actual.Data["cwagentconfig.json"])
		assert.YAMLEq(t, expectedData["cwagentotelconfig.yaml"], actual.Data["cwagentotelconfig.yaml"])
	})
}
