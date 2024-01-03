// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameOpenCensus = "__opencensus"

// NewOpenCensusReceiverParser builds a new parser for OpenCensus receivers.
func NewOpenCensusReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 55678,
		parserName:  parserNameOpenCensus,
	}
}

func init() {
	Register("opencensus", NewOpenCensusReceiverParser)
}
