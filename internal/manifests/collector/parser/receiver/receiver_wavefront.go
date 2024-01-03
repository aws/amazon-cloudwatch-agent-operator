// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameWavefront = "__wavefront"

// NewWavefrontReceiverParser builds a new parser for Wavefront receivers, from the contrib repository.
func NewWavefrontReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 2003,
		parserName:  parserNameWavefront,
	}
}

func init() {
	Register("wavefront", NewWavefrontReceiverParser)
}
