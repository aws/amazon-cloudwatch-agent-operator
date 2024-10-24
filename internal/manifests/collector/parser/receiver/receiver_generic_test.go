// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver_test

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser/receiver"
)

var logger = logf.Log.WithName("unit-tests")

func TestParseEndpoint(t *testing.T) {
	// prepare
	// there's no parser registered to handle "myreceiver", so, it falls back to the generic parser
	builder := receiver.NewGenericReceiverParser(logger, "myreceiver", map[interface{}]interface{}{
		"endpoint": "0.0.0.0:1234",
	})

	// test
	ports, err := builder.Ports()

	// verify
	assert.NoError(t, err)
	assert.Len(t, ports, 1)
	assert.EqualValues(t, 1234, ports[0].Port)
}

func TestFailedToParseEndpoint(t *testing.T) {
	// prepare
	// there's no parser registered to handle "myreceiver", so, it falls back to the generic parser
	builder := receiver.NewGenericReceiverParser(logger, "myreceiver", map[interface{}]interface{}{
		"endpoint": "0.0.0.0",
	})

	// test
	ports, err := builder.Ports()

	// verify
	assert.NoError(t, err)
	assert.Len(t, ports, 0)
}

func TestDownstreamParsers(t *testing.T) {
	for _, tt := range []struct {
		builder      func(logr.Logger, string, map[interface{}]interface{}) parser.ComponentPortParser
		desc         string
		receiverName string
		parserName   string
		defaultPort  int
	}{
		{receiver.NewZipkinReceiverParser, "zipkin", "zipkin", "__zipkin", 9411},

		// contrib receivers
		{receiver.NewStatsdReceiverParser, "statsd", "statsd", "__statsd", 8125},
		{receiver.NewAWSXrayReceiverParser, "awsxray", "awsxray", "__awsxray", 2000},
	} {
		t.Run(tt.receiverName, func(t *testing.T) {
			t.Run("builds successfully", func(t *testing.T) {
				// test
				builder := tt.builder(logger, tt.receiverName, map[interface{}]interface{}{})

				// verify
				assert.Equal(t, tt.parserName, builder.ParserName())
			})

			t.Run("assigns the expected port", func(t *testing.T) {
				// prepare
				builder := tt.builder(logger, tt.receiverName, map[interface{}]interface{}{})

				// test
				ports, err := builder.Ports()

				// verify
				assert.NoError(t, err)
				assert.Len(t, ports, 1)
				assert.EqualValues(t, tt.defaultPort, ports[0].Port)
				assert.Equal(t, tt.receiverName, ports[0].Name)
			})

			t.Run("allows port to be overridden", func(t *testing.T) {
				// prepare
				builder := tt.builder(logger, tt.receiverName, map[interface{}]interface{}{
					"endpoint": "0.0.0.0:65535",
				})

				// test
				ports, err := builder.Ports()

				// verify
				assert.NoError(t, err)
				assert.Len(t, ports, 1)
				assert.EqualValues(t, 65535, ports[0].Port)
				assert.Equal(t, tt.receiverName, ports[0].Name)
			})
		})
	}
}
