// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package config contains the operator's runtime configuration.
package config

import (
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

const (
	defaultCollectorConfigMapEntry       = "cwagentconfig.json"
	defaultOtelCollectorConfigMapEntry   = "cwagentotelconfig.yaml"
	defaultTargetAllocatorConfigMapEntry = "targetallocator.yaml"
	defaultPrometheusConfigMapEntry      = "prometheus.yaml"
)

// Config holds the static configuration for this operator.
type Config struct {
	logger                              logr.Logger
	autoInstrumentationPythonImage      string
	collectorImage                      string
	collectorConfigMapEntry             string
	otelCollectorConfigMapEntry         string
	autoInstrumentationDotNetImage      string
	autoInstrumentationGoImage          string
	autoInstrumentationApacheHttpdImage string
	autoInstrumentationNginxImage       string
	autoInstrumentationNodeJSImage      string
	autoInstrumentationJavaImage        string
	dcgmExporterImage                   string
	neuronMonitorImage                  string
	targetAllocatorImage                string
	targetAllocatorConfigMapEntry       string
	prometheusConfigMapEntry            string
	labelsFilter                        []string
}

// New constructs a new configuration based on the given options.
func New(opts ...Option) Config {
	// initialize with the default values
	o := options{
		collectorConfigMapEntry:       defaultCollectorConfigMapEntry,
		otelCollectorConfigMapEntry:   defaultOtelCollectorConfigMapEntry,
		targetAllocatorConfigMapEntry: defaultTargetAllocatorConfigMapEntry,
		prometheusConfigMapEntry:      defaultPrometheusConfigMapEntry,
		logger:                        logf.Log.WithName("config"),
		version:                       version.Get(),
	}
	for _, opt := range opts {
		opt(&o)
	}

	return Config{
		collectorImage:                      o.collectorImage,
		collectorConfigMapEntry:             o.collectorConfigMapEntry,
		otelCollectorConfigMapEntry:         o.otelCollectorConfigMapEntry,
		logger:                              o.logger,
		autoInstrumentationJavaImage:        o.autoInstrumentationJavaImage,
		autoInstrumentationNodeJSImage:      o.autoInstrumentationNodeJSImage,
		autoInstrumentationPythonImage:      o.autoInstrumentationPythonImage,
		autoInstrumentationDotNetImage:      o.autoInstrumentationDotNetImage,
		autoInstrumentationGoImage:          o.autoInstrumentationGoImage,
		autoInstrumentationApacheHttpdImage: o.autoInstrumentationApacheHttpdImage,
		autoInstrumentationNginxImage:       o.autoInstrumentationNginxImage,
		dcgmExporterImage:                   o.dcgmExporterImage,
		neuronMonitorImage:                  o.neuronMonitorImage,
		targetAllocatorImage:                o.targetAllocatorImage,
		targetAllocatorConfigMapEntry:       o.targetAllocatorConfigMapEntry,
		prometheusConfigMapEntry:            o.prometheusConfigMapEntry,
		labelsFilter:                        o.labelsFilter,
	}
}

// CollectorImage represents the flag to override the OpenTelemetry Collector container image.
func (c *Config) CollectorImage() string {
	return c.collectorImage
}

// CollectorConfigMapEntry represents the configuration JSON file name for the collector. Immutable.
func (c *Config) CollectorConfigMapEntry() string {
	return c.collectorConfigMapEntry
}

// OtelCollectorConfigMapEntry represents the configuration YAML file name for the collector. Immutable.
func (c *Config) OtelCollectorConfigMapEntry() string {
	return c.otelCollectorConfigMapEntry
}

// AutoInstrumentationJavaImage returns OpenTelemetry Java auto-instrumentation container image.
func (c *Config) AutoInstrumentationJavaImage() string {
	return c.autoInstrumentationJavaImage
}

// AutoInstrumentationNodeJSImage returns OpenTelemetry NodeJS auto-instrumentation container image.
func (c *Config) AutoInstrumentationNodeJSImage() string {
	return c.autoInstrumentationNodeJSImage
}

// AutoInstrumentationPythonImage returns OpenTelemetry Python auto-instrumentation container image.
func (c *Config) AutoInstrumentationPythonImage() string {
	return c.autoInstrumentationPythonImage
}

// AutoInstrumentationDotNetImage returns OpenTelemetry DotNet auto-instrumentation container image.
func (c *Config) AutoInstrumentationDotNetImage() string {
	return c.autoInstrumentationDotNetImage
}

// AutoInstrumentationGoImage returns OpenTelemetry Go auto-instrumentation container image.
func (c *Config) AutoInstrumentationGoImage() string {
	return c.autoInstrumentationGoImage
}

// AutoInstrumentationApacheHttpdImage returns OpenTelemetry ApacheHttpd auto-instrumentation container image.
func (c *Config) AutoInstrumentationApacheHttpdImage() string {
	return c.autoInstrumentationApacheHttpdImage
}

// AutoInstrumentationNginxImage returns OpenTelemetry Nginx auto-instrumentation container image.
func (c *Config) AutoInstrumentationNginxImage() string {
	return c.autoInstrumentationNginxImage
}

// DcgmExporterImage returns Nvidia DCGM Exporter container image.
func (c *Config) DcgmExporterImage() string {
	return c.dcgmExporterImage
}

// NeuronMonitorImage returns Neuron Monitor Exporter container image.
func (c *Config) NeuronMonitorImage() string {
	return c.neuronMonitorImage
}

// TargetAllocatorImage represents the flag to override the OpenTelemetry TargetAllocator container image.
func (c *Config) TargetAllocatorImage() string {
	return c.targetAllocatorImage
}

// TargetAllocatorConfigMapEntry represents the configuration file name for the TargetAllocator. Immutable.
func (c *Config) TargetAllocatorConfigMapEntry() string {
	return c.targetAllocatorConfigMapEntry
}

// PrometheusConfigMapEntry represents the configuration file name for Prometheus.
func (c *Config) PrometheusConfigMapEntry() string { return c.prometheusConfigMapEntry }

// LabelsFilter Returns the filters converted to regex strings used to filter out unwanted labels from propagations.
func (c *Config) LabelsFilter() []string {
	return c.labelsFilter
}
