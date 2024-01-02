// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/autodetect"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/autodetect/openshift"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestNewConfig(t *testing.T) {
	// prepare
	cfg := config.New(
		config.WithCollectorImage("some-image"),
		config.WithCollectorConfigMapEntry("some-config.yaml"),
		config.WithOpenShiftRoutesAvailability(openshift.RoutesAvailable),
	)

	// test
	assert.Equal(t, "some-image", cfg.CollectorImage())
	assert.Equal(t, "some-config.yaml", cfg.CollectorConfigMapEntry())
	assert.Equal(t, openshift.RoutesAvailable, cfg.OpenShiftRoutesAvailability())
}

func TestConfigChangesOnAutoDetect(t *testing.T) {
	// prepare
	mock := &mockAutoDetect{
		OpenShiftRoutesAvailabilityFunc: func() (openshift.RoutesAvailability, error) {
			return openshift.RoutesAvailable, nil
		},
	}
	cfg := config.New(
		config.WithAutoDetect(mock),
	)

	// sanity check
	require.Equal(t, openshift.RoutesNotAvailable, cfg.OpenShiftRoutesAvailability())

	// test
	err := cfg.AutoDetect()
	require.NoError(t, err)

	// verify
	assert.Equal(t, openshift.RoutesAvailable, cfg.OpenShiftRoutesAvailability())
}

var _ autodetect.AutoDetect = (*mockAutoDetect)(nil)

type mockAutoDetect struct {
	OpenShiftRoutesAvailabilityFunc func() (openshift.RoutesAvailability, error)
}

func (m *mockAutoDetect) OpenShiftRoutesAvailability() (openshift.RoutesAvailability, error) {
	if m.OpenShiftRoutesAvailabilityFunc != nil {
		return m.OpenShiftRoutesAvailabilityFunc()
	}
	return openshift.RoutesNotAvailable, nil
}
