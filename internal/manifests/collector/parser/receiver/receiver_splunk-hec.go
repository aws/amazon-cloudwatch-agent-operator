// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameSplunkHec = "__splunk_hec"

// NewSplunkHecReceiverParser builds a new parser for Splunk Hec receivers, from the contrib repository.
func NewSplunkHecReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 8088,
		parserName:  parserNameSplunkHec,
	}
}

func init() {
	Register("splunk_hec", NewSplunkHecReceiverParser)
}
