// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

var _ parser.ComponentPortParser = &PrometheusExporterParser{}

const (
	parserNamePrometheus  = "__prometheus"
	defaultPrometheusPort = 8888
)

// PrometheusExporterParser parses the configuration for OTLP receivers.
type PrometheusExporterParser struct {
	config map[interface{}]interface{}
	logger logr.Logger
	name   string
}

// NewPrometheusExporterParser builds a new parser for OTLP receivers.
func NewPrometheusExporterParser(logger logr.Logger, name string, config map[interface{}]interface{}) parser.ComponentPortParser {
	return &PrometheusExporterParser{
		logger: logger,
		name:   name,
		config: config,
	}
}

// Ports returns all the service ports for all protocols in this parser.
func (o *PrometheusExporterParser) Ports() ([]corev1.ServicePort, error) {
	ports := []corev1.ServicePort{}
	if o.config == nil {
		ports = append(ports,
			corev1.ServicePort{
				Name:       naming.PortName(o.name, defaultPrometheusPort),
				Port:       defaultPrometheusPort,
				TargetPort: intstr.FromInt(int(defaultPrometheusPort)),
				Protocol:   corev1.ProtocolTCP,
			},
		)
	} else {
		ports = append(
			ports, *singlePortFromConfigEndpoint(o.logger, o.name, o.config),
		)
	}

	return ports, nil
}

// ParserName returns the name of this parser.
func (o *PrometheusExporterParser) ParserName() string {
	return parserNamePrometheus
}

func init() {
	Register("prometheus", NewPrometheusExporterParser)
}
