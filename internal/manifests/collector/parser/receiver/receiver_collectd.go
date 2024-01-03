// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameCollectd = "__collectd"

// NewCollectdReceiverParser builds a new parser for Collectd receivers, from the contrib repository.
func NewCollectdReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 8081,
		parserName:  parserNameCollectd,
	}
}

func init() {
	Register("collectd", NewCollectdReceiverParser)
}
