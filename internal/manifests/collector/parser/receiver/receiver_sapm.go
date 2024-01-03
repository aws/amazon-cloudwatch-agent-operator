// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameSAPM = "__sapm"

// NewSAPMReceiverParser builds a new parser for SAPM receivers, from the contrib repository.
func NewSAPMReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 7276,
		parserName:  parserNameSAPM,
	}
}

func init() {
	Register("sapm", NewSAPMReceiverParser)
}
