// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameSignalFx = "__signalfx"

// NewSignalFxReceiverParser builds a new parser for SignalFx receivers, from the contrib repository.
func NewSignalFxReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 9943,
		parserName:  parserNameSignalFx,
	}
}

func init() {
	Register("signalfx", NewSignalFxReceiverParser)
}
