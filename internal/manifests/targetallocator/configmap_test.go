// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.my-instance",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}
	collector := collectorInstance()
	targetAllocator := targetAllocatorInstance()
	cfg := config.New()
	params := Params{
		Collector:       collector,
		TargetAllocator: targetAllocator,
		Config:          cfg,
		Log:             logr.Discard(),
	}

	t.Run("should return expected target allocator config map", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-targetallocator"
		expectedLabels["app.kubernetes.io/name"] = "my-instance-targetallocator"

		expectedData := map[string]string{
			targetAllocatorFilename: `allocation_strategy: consistent-hashing
collector_selector:
  matchlabels:
    app.kubernetes.io/component: amazon-cloudwatch-agent
    app.kubernetes.io/instance: default.my-instance
    app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
    app.kubernetes.io/part-of: amazon-cloudwatch-agent
  matchexpressions: []
config:
  scrape_configs:
  - job_name: amazon-cloudwatch-agent
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
filter_strategy: relabel-config
`,
		}

		actual, err := ConfigMap(params)
		require.NoError(t, err)

		assert.Equal(t, "my-instance-targetallocator", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData[targetAllocatorFilename], actual.Data[targetAllocatorFilename])

	})
	t.Run("should return target allocator config map without scrape configs", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-targetallocator"
		expectedLabels["app.kubernetes.io/name"] = "my-instance-targetallocator"

		expectedData := map[string]string{
			targetAllocatorFilename: `allocation_strategy: consistent-hashing
collector_selector:
  matchlabels:
    app.kubernetes.io/component: amazon-cloudwatch-agent-collector
    app.kubernetes.io/instance: default.my-instance
    app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
    app.kubernetes.io/part-of: amazon-cloudwatch-agent
  matchexpressions: []
filter_strategy: relabel-config
`,
		}
		targetAllocator = targetAllocatorInstance()
		targetAllocator.Spec.ScrapeConfigs = []v1alpha1.AnyConfig{}
		params.TargetAllocator = targetAllocator
		actual, err := ConfigMap(params)
		require.NoError(t, err)

		assert.Equal(t, "my-instance-targetallocator", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData[targetAllocatorFilename], actual.Data[targetAllocatorFilename])

	})
	t.Run("should return expected target allocator config map with label selectors", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-targetallocator"
		expectedLabels["app.kubernetes.io/name"] = "my-instance-targetallocator"

		expectedData := map[string]string{
			targetAllocatorFilename: `allocation_strategy: consistent-hashing
collector_selector:
  matchlabels:
    app.kubernetes.io/component: amazon-cloudwatch-agent-collector
    app.kubernetes.io/instance: default.my-instance
    app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
    app.kubernetes.io/part-of: amazon-cloudwatch-agent
  matchexpressions: []
config:
  scrape_configs:
  - job_name: otel-collector
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
filter_strategy: relabel-config
prometheus_cr:
  enabled: true
  pod_monitor_selector:
    matchlabels:
      release: my-instance
    matchexpressions: []
  service_monitor_selector:
    matchlabels:
      release: my-instance
    matchexpressions: []
`,
		}
		targetAllocator = targetAllocatorInstance()
		targetAllocator.Spec.PrometheusCR.Enabled = true
		targetAllocator.Spec.PrometheusCR.PodMonitorSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"release": "my-instance",
			},
		}
		targetAllocator.Spec.PrometheusCR.ServiceMonitorSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"release": "my-instance",
			}}
		params.TargetAllocator = targetAllocator
		actual, err := ConfigMap(params)
		assert.NoError(t, err)

		assert.Equal(t, "my-instance-targetallocator", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
	t.Run("should return expected target allocator config map with scrape interval set", func(t *testing.T) {
		expectedLabels["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-targetallocator"
		expectedLabels["app.kubernetes.io/name"] = "my-instance-targetallocator"

		expectedData := map[string]string{
			targetAllocatorFilename: `allocation_strategy: consistent-hashing
collector_selector:
  matchlabels:
    app.kubernetes.io/component: amazon-cloudwatch-agent-collector
    app.kubernetes.io/instance: default.my-instance
    app.kubernetes.io/managed-by: amazon-cloudwatch-agent-operator
    app.kubernetes.io/part-of: amazon-cloudwatch-agent
  matchexpressions: []
config:
  scrape_configs:
  - job_name: otel-collector
    scrape_interval: 10s
    static_configs:
    - targets:
      - 0.0.0.0:8888
      - 0.0.0.0:9999
filter_strategy: relabel-config
prometheus_cr:
  enabled: true
  pod_monitor_selector: null
  scrape_interval: 30s
  service_monitor_selector: null
`,
		}

		targetAllocator = targetAllocatorInstance()
		targetAllocator.Spec.PrometheusCR.Enabled = true
		targetAllocator.Spec.PrometheusCR.ScrapeInterval = &metav1.Duration{Duration: time.Second * 30}
		params.TargetAllocator = targetAllocator
		actual, err := ConfigMap(params)
		assert.NoError(t, err)

		assert.Equal(t, "my-instance-targetallocator", actual.Name)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})

}
