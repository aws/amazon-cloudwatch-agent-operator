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
		"app.kubernetes.io/part-of":    "opentelemetry",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected collector config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "opentelemetry-collector"
		expectedLables["app.kubernetes.io/name"] = "test-collector"
		expectedLables["app.kubernetes.io/version"] = "0.47.0"

		expectedData := map[string]string{
			"collector.yaml": `processors:
receivers:
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
      processors: []
      exporters: [debug]`,
		}

		param := deploymentParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-collector", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}
