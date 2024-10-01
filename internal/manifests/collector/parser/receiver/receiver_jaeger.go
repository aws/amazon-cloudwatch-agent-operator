// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

var _ parser.ComponentPortParser = &JaegerReceiverParser{}

const (
	parserNameJaeger = "__jaeger"

	defaultGRPCPort          int32 = 14250
	defaultThriftHTTPPort    int32 = 14268
	defaultThriftCompactPort int32 = 6831
	defaultThriftBinaryPort  int32 = 6832
)

// JaegerReceiverParser parses the configuration for Jaeger-specific receivers.
type JaegerReceiverParser struct {
	config map[interface{}]interface{}
	logger logr.Logger
	name   string
}

// NewJaegerReceiverParser builds a new parser for Jaeger receivers.
func NewJaegerReceiverParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	if protocols, ok := config["protocols"].(map[interface{}]interface{}); ok {
		return &JaegerReceiverParser{
			logger: logger,
			name:   name,
			config: protocols,
		}
	}

	return &JaegerReceiverParser{
		name:   name,
		config: map[interface{}]interface{}{},
	}
}

// Ports returns all the service ports for all protocols in this parser.
func (j *JaegerReceiverParser) Ports() ([]corev1.ServicePort, error) {
	ports := []corev1.ServicePort{}

	for _, protocol := range []struct {
		name              string
		transportProtocol corev1.Protocol
		appProtocol       string
		defaultPort       int32
	}{
		{
			name:              "grpc",
			defaultPort:       defaultGRPCPort,
			transportProtocol: corev1.ProtocolTCP,
			appProtocol:       "grpc",
		},
		{
			name:              "thrift_http",
			defaultPort:       defaultThriftHTTPPort,
			transportProtocol: corev1.ProtocolTCP,
			appProtocol:       "http",
		},
		{
			name:              "thrift_compact",
			defaultPort:       defaultThriftCompactPort,
			transportProtocol: corev1.ProtocolUDP,
		},
		{
			name:              "thrift_binary",
			defaultPort:       defaultThriftBinaryPort,
			transportProtocol: corev1.ProtocolUDP,
		},
	} {
		// do we have the protocol specified at all?
		if receiverProtocol, ok := j.config[protocol.name]; ok {
			// we have the specified protocol, we definitely need a service port
			nameWithProtocol := fmt.Sprintf("%s-%s", j.name, protocol.name)
			var protocolPort *corev1.ServicePort

			// do we have a configuration block for the protocol?
			settings, ok := receiverProtocol.(map[interface{}]interface{})
			if ok {
				protocolPort = singlePortFromConfigEndpoint(j.logger, nameWithProtocol, settings)
			}

			// have we parsed a port based on the configuration block?
			// if not, we use the default port
			if protocolPort == nil {
				protocolPort = &corev1.ServicePort{
					Name: naming.PortName(nameWithProtocol, protocol.defaultPort),
					Port: protocol.defaultPort,
				}
			}

			// set the appropriate transport protocol (i.e. TCP/UDP) for this kind of receiver protocol
			protocolPort.Protocol = protocol.transportProtocol

			if protocol.appProtocol != "" {
				c := protocol.appProtocol
				protocolPort.AppProtocol = &c
			}

			// at this point, we *have* a port specified, add it to the list of ports
			ports = append(ports, *protocolPort)
		}
	}

	return ports, nil
}

// ParserName returns the name of this parser.
func (j *JaegerReceiverParser) ParserName() string {
	return parserNameJaeger
}

func init() {
	Register("jaeger", NewJaegerReceiverParser)
}
