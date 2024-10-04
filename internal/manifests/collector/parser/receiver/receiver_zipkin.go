// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameZipkin = "__zipkin"

// NewZipkinReceiverParser builds a new parser for Zipkin receivers.
func NewZipkinReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	http := "http"
	return &GenericReceiver{
		logger:             logger,
		name:               name,
		config:             config,
		defaultPort:        9411,
		defaultProtocol:    corev1.ProtocolTCP,
		defaultAppProtocol: &http,
		parserName:         parserNameZipkin,
	}
}

func init() {
	Register("zipkin", NewZipkinReceiverParser)
}
