// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameStatsd = "__statsd"

// NewStatsdReceiverParser builds a new parser for Statsd receivers, from the contrib repository.
func NewStatsdReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:          logger,
		name:            name,
		config:          config,
		defaultPort:     8125,
		defaultProtocol: corev1.ProtocolUDP,
		parserName:      parserNameStatsd,
	}
}

func init() {
	Register("statsd", NewStatsdReceiverParser)
}
