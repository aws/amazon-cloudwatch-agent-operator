// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesiredServiceMonitors(t *testing.T) {
	params := deploymentParams()

	actual, err := ServiceMonitor(params)
	assert.NoError(t, err)
	assert.Nil(t, actual)

	params.OtelCol.Spec.Observability.Metrics.EnableMetrics = true
	actual, err = ServiceMonitor(params)
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-collector", params.OtelCol.Name), actual.Name)
	assert.Equal(t, params.OtelCol.Namespace, actual.Namespace)
	assert.Equal(t, "monitoring", actual.Spec.Endpoints[0].Port)

	params, err = newParams("", "testdata/prometheus-exporter.yaml")
	assert.NoError(t, err)
	params.OtelCol.Spec.Observability.Metrics.EnableMetrics = true
	actual, err = ServiceMonitor(params)
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-collector", params.OtelCol.Name), actual.Name)
	assert.Equal(t, params.OtelCol.Namespace, actual.Namespace)
	assert.Equal(t, "monitoring", actual.Spec.Endpoints[0].Port)
	assert.Equal(t, "prometheus-dev", actual.Spec.Endpoints[1].Port)
	assert.Equal(t, "prometheus-prod", actual.Spec.Endpoints[2].Port)
}
