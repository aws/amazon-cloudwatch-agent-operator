// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.my-instance",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected target allocator config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-target-allocator"
		expectedLables["app.kubernetes.io/name"] = "my-instance-target-allocator"

		expectedData := map[string]string{
			"target-allocator.yaml": `allocation_strategy: consistent-hashing
config:
  scrape_configs:
  - job_name: otel-collector
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
label_selector:
  app.kubernetes.io/component: amazon-cloudwatch-agent
  app.kubernetes.io/instance: default.my-instance
  app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
  app.kubernetes.io/part-of: amazon-cloudwatch-agent
`,
		}
		instance := collectorInstance()
		cfg := config.New()
		params := manifests.Params{
			OtelCol: instance,
			Config:  cfg,
			Log:     logr.Discard(),
		}
		actual, err := ConfigMap(params)
		assert.NoError(t, err)

		assert.Equal(t, "my-instance-target-allocator", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
	t.Run("should return expected target allocator config map with label selectors", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-target-allocator"
		expectedLables["app.kubernetes.io/name"] = "my-instance-target-allocator"

		expectedData := map[string]string{
			"target-allocator.yaml": `allocation_strategy: consistent-hashing
config:
  scrape_configs:
  - job_name: otel-collector
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
label_selector:
  app.kubernetes.io/component: amazon-cloudwatch-agent
  app.kubernetes.io/instance: default.my-instance
  app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
  app.kubernetes.io/part-of: amazon-cloudwatch-agent
pod_monitor_selector:
  release: my-instance
service_monitor_selector:
  release: my-instance
`,
		}
		instance := collectorInstance()
		instance.Spec.TargetAllocator.PrometheusCR.PodMonitorSelector = map[string]string{
			"release": "my-instance",
		}
		instance.Spec.TargetAllocator.PrometheusCR.ServiceMonitorSelector = map[string]string{
			"release": "my-instance",
		}
		cfg := config.New()
		params := manifests.Params{
			OtelCol: instance,
			Config:  cfg,
			Log:     logr.Discard(),
		}
		actual, err := ConfigMap(params)
		assert.NoError(t, err)

		assert.Equal(t, "my-instance-target-allocator", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
	t.Run("should return expected target allocator config map with scrape interval set", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-target-allocator"
		expectedLables["app.kubernetes.io/name"] = "my-instance-target-allocator"

		expectedData := map[string]string{
			"target-allocator.yaml": `allocation_strategy: consistent-hashing
config:
  scrape_configs:
  - job_name: otel-collector
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
label_selector:
  app.kubernetes.io/component: amazon-cloudwatch-agent
  app.kubernetes.io/instance: default.my-instance
  app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
  app.kubernetes.io/part-of: amazon-cloudwatch-agent
prometheus_cr:
  scrape_interval: 30s
`,
		}

		collector := collectorInstance()
		collector.Spec.TargetAllocator.PrometheusCR.ScrapeInterval = &metav1.Duration{Duration: time.Second * 30}
		cfg := config.New()
		params := manifests.Params{
			OtelCol: collector,
			Config:  cfg,
			Log:     logr.Discard(),
		}
		actual, err := ConfigMap(params)
		assert.NoError(t, err)

		assert.Equal(t, "my-instance-target-allocator", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})

}
