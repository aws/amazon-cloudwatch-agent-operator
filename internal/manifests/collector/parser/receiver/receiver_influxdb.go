// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameInfluxdb = "__influxdb"

// NewInfluxdbReceiverParser builds a new parser for Influxdb receivers, from the contrib repository.
func NewInfluxdbReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 8086,
		parserName:  parserNameInfluxdb,
	}
}

func init() {
	Register("influxdb", NewInfluxdbReceiverParser)
}
