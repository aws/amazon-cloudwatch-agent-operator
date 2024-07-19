// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build ignore_test

package collector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func sidecarParams() manifests.Params {
	return paramsWithMode(v1alpha1.ModeSidecar)
}

func TestDesiredPodMonitors(t *testing.T) {
	params := sidecarParams()

	actual, err := PodMonitor(params)
	assert.NoError(t, err)
	assert.Nil(t, actual)

	params.OtelCol.Spec.Observability.Metrics.EnableMetrics = true
	actual, err = PodMonitor(params)
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-collector", params.OtelCol.Name), actual.Name)
	assert.Equal(t, params.OtelCol.Namespace, actual.Namespace)
	assert.Equal(t, "monitoring", actual.Spec.PodMetricsEndpoints[0].Port)
}

func TestDesiredPodMonitorsWithPrometheus(t *testing.T) {
	params, err := newParams("", "testdata/prometheus-exporter.yaml")
	assert.NoError(t, err)
	params.OtelCol.Spec.Mode = v1alpha1.ModeSidecar
	params.OtelCol.Spec.Observability.Metrics.EnableMetrics = true
	actual, err := PodMonitor(params)
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-collector", params.OtelCol.Name), actual.Name)
	assert.Equal(t, params.OtelCol.Namespace, actual.Namespace)
	assert.Equal(t, "monitoring", actual.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, "prometheus-dev", actual.Spec.PodMetricsEndpoints[1].Port)
	assert.Equal(t, "prometheus-prod", actual.Spec.PodMetricsEndpoints[2].Port)
}
