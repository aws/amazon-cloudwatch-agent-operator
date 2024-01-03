// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	// prepare

	// First Test - Exporters
	configStr := `
receivers:
  httpd/mtls:
    protocols:
      http:
        endpoint: mysite.local:55690
  jaeger:
    protocols:
      grpc:
  prometheus:
    protocols:
      grpc:

processors:

exporters:
  debug:

service:
  pipelines:
    metrics:
      receivers: [httpd/mtls, jaeger]
      exporters: [debug]
    metrics/1:
      receivers: [httpd/mtls, jaeger]
      exporters: [debug]
`
	// // prepare
	config, err := ConfigFromString(configStr)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	// test
	check := getEnabledComponents(config, ComponentTypeReceiver)
	require.NotEmpty(t, check)
}

func TestEmptyEnabledReceivers(t *testing.T) {
	// prepare

	// First Test - Exporters
	configStr := `
receivers:
  httpd/mtls:
    protocols:
      http:
        endpoint: mysite.local:55690
  jaeger:
    protocols:
      grpc:
  prometheus:
    protocols:
      grpc:

processors:

exporters:
  debug:

service:
  pipelines:
    metrics:
      receivers: []
      exporters: []
    metrics/1:
      receivers: []
      exporters: []
`
	// // prepare
	config, err := ConfigFromString(configStr)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	// test
	check := getEnabledComponents(config, ComponentTypeReceiver)
	require.Empty(t, check)
}
