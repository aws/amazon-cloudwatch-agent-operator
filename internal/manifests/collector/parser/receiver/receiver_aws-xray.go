// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
)

const parserNameAWSXRAY = "__awsxray"

// NewAWSXrayReceiverParser builds a new parser for AWS xray receivers, from the contrib repository.
func NewAWSXrayReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:      logger,
		name:        name,
		config:      config,
		defaultPort: 2000,
		parserName:  parserNameAWSXRAY,
	}
}

func init() {
	Register("awsxray", NewAWSXrayReceiverParser)
}
