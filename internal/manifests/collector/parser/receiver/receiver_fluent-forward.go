// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameFluentForward = "__fluentforward"

// NewFluentForwardReceiverParser builds a new parser for FluentForward receivers, from the contrib repository.
func NewFluentForwardReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 8006,
		parserName:  parserNameFluentForward,
	}
}

func init() {
	Register("fluentforward", NewFluentForwardReceiverParser)
}
