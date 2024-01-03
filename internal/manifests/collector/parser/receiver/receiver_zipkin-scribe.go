// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameZipkinScribe = "__zipkinscribe"

// NewZipkinScribeReceiverParser builds a new parser for ZipkinScribe receivers.
func NewZipkinScribeReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 9410,
		parserName:  parserNameZipkinScribe,
	}
}

func init() {
	Register("zipkin-scribe", NewZipkinScribeReceiverParser)
}
