// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

const parserNameGeneric = "__generic"

var _ parser.ComponentPortParser = &GenericReceiver{}

// GenericReceiver is a special parser for generic receivers. It doesn't self-register and should be created/used directly.
type GenericReceiver struct {
	config             map[interface{}]interface{}
	defaultAppProtocol *string
	logger             logr.Logger
	name               string
	defaultProtocol    corev1.Protocol
	parserName         string
	defaultPort        int32
}

// NOTE: Operator will sync with only receivers that aren't scrapers. Operator sync up receivers
// so that it can expose the required port based on the receiver's config. Receiver scrapers are ignored.

// NewGenericReceiverParser builds a new parser for generic receivers.
func NewGenericReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &GenericReceiver{
		logger:     logger,
		name:       name,
		config:     config,
		parserName: parserNameGeneric,
	}
}

// Ports returns all the service ports for all protocols in this parser.
func (g *GenericReceiver) Ports() ([]corev1.ServicePort, error) {
	port := singlePortFromConfigEndpoint(g.logger, g.name, g.config)
	if port != nil {
		port.Protocol = g.defaultProtocol
		port.AppProtocol = g.defaultAppProtocol
		return []corev1.ServicePort{*port}, nil
	}

	if g.defaultPort > 0 {
		return []corev1.ServicePort{{
			Port:        g.defaultPort,
			Name:        naming.PortName(g.name, g.defaultPort),
			Protocol:    g.defaultProtocol,
			AppProtocol: g.defaultAppProtocol,
		}}, nil
	}

	return []corev1.ServicePort{}, nil
}

// ParserName returns the name of this parser.
func (g *GenericReceiver) ParserName() string {
	return g.parserName
}
