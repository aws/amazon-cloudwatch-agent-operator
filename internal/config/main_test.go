// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestNewConfig(t *testing.T) {
	// prepare
	cfg := config.New(
		config.WithCollectorImage("some-image"),
		config.WithCollectorConfigMapEntry("some-config.yaml"),
		config.WithTargetAllocatorConfigMapEntry("some-ta-config.yaml"),
		config.WithPrometheusConfigMapEntry("some-prom-config.yaml"),
	)

	// test
	assert.Equal(t, "some-image", cfg.CollectorImage())
	assert.Equal(t, "some-config.yaml", cfg.CollectorConfigMapEntry())
	assert.Equal(t, "some-ta-config.yaml", cfg.TargetAllocatorConfigMapEntry())
	assert.Equal(t, "some-prom-config.yaml", cfg.PrometheusConfigMapEntry())
}
