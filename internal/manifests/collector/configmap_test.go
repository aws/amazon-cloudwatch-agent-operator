// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected collector config map", func(t *testing.T) {

		expectedData := map[string]string{
			"collector.yaml": `receivers:
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

		param := deploymentParams()

		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-collector"
		expectedLables["app.kubernetes.io/name"] = "test-collector"
		expectedLables["app.kubernetes.io/version"] = "0.47.0"

		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, len(expectedData), len(actual.Data))
		for k, expected := range expectedData {
			assert.YAMLEq(t, expected, actual.Data[k])
		}
	})

	t.Run("should return expected escaped collector config map with target_allocator config block", func(t *testing.T) {
		expectedData := map[string]string{
			"collector.yaml": `exporters:
  debug:
receivers:
  prometheus:
    config: {}
    target_allocator:
      collector_id: ${POD_NAME}
      endpoint: http://test-targetallocator.default.svc.cluster.local:80
      interval: 30s
service:
  pipelines:
    metrics:
      exporters:
      - debug
      receivers:
      - prometheus
`,
		}

		param, err := newParams("test/test-img", "testdata/http_sd_config_servicemonitor_test.yaml")
		assert.NoError(t, err)

		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-collector"
		expectedLables["app.kubernetes.io/name"] = "test-collector"
		expectedLables["app.kubernetes.io/version"] = "latest"

		param.OtelCol.Spec.TargetAllocator.Enabled = true
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, len(expectedData), len(actual.Data))
		for k, expected := range expectedData {
			assert.YAMLEq(t, expected, actual.Data[k])
		}

		// Reset the value
		expectedLables["app.kubernetes.io/version"] = "0.47.0"
		assert.NoError(t, err)

	})

}
