package adapters

import (
	"testing"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stretchr/testify/require"
)

var logger = logf.Log.WithName("unit-tests")

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
  logging:

service:
  pipelines:
    metrics:
      receivers: [httpd/mtls, jaeger]
      exporters: [logging]
    metrics/1:
      receivers: [httpd/mtls, jaeger]
      exporters: [logging]
`
	// // prepare
	config, err := ConfigFromString(configStr)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	// test
	check := GetEnabledReceivers(logger, config)
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
  logging:

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
	check := GetEnabledReceivers(logger, config)
	require.Empty(t, check)
}
