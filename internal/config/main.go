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
	defaultCollectorConfigMapEntry = "cwagentconfig.json"
	defaultTargetAllocatorConfigMapEntry = "targetallocator.yaml"
)

// Config holds the static configuration for this operator.
type Config struct {
	logger                              logr.Logger
	autoInstrumentationPythonImage      string
	collectorImage                      string
	collectorConfigMapEntry             string
	autoInstrumentationDotNetImage      string
	autoInstrumentationGoImage          string
	autoInstrumentationApacheHttpdImage string
	autoInstrumentationNginxImage       string
	autoInstrumentationNodeJSImage      string
	autoInstrumentationJavaImage        string
	targetAllocatorImage          string
	targetAllocatorConfigMapEntry string
	dcgmExporterImage                   string
	neuronMonitorImage                  string
	labelsFilter                        []string
}

// New constructs a new configuration based on the given options.
func New(opts ...Option) Config {
	// initialize with the default values
	o := options{
		collectorConfigMapEntry: defaultCollectorConfigMapEntry,
		targetAllocatorConfigMapEntry: defaultTargetAllocatorConfigMapEntry,logger:                  logf.Log.WithName("config"),
		version:                       version.Get(),
	}
	for _, opt := range opts {
		opt(&o)
	}

	// this is derived from another option, so, we need to first parse the options, then set a default
	// if there's no explicit value being set
	if len(o.collectorImage) == 0 {
		o.collectorImage = fmt.Sprintf("otel/opentelemetry-collector:%s", o.version.OpenTelemetryCollector)
	}

	if len(o.targetAllocatorImage) == 0 {
		o.targetAllocatorImage = fmt.Sprintf("quay.io/opentelemetry/target-allocator:%s", o.version.TargetAllocator)
	}

	return Config{
		collectorImage:                      o.collectorImage,
		collectorConfigMapEntry:             o.collectorConfigMapEntry,
		targetAllocatorImage:          o.targetAllocatorImage,
		targetAllocatorConfigMapEntry: o.targetAllocatorConfigMapEntry,logger:                              o.logger,
		autoInstrumentationJavaImage:        o.autoInstrumentationJavaImage,
		autoInstrumentationNodeJSImage:      o.autoInstrumentationNodeJSImage,
		autoInstrumentationPythonImage:      o.autoInstrumentationPythonImage,
		autoInstrumentationDotNetImage:      o.autoInstrumentationDotNetImage,
		autoInstrumentationGoImage:          o.autoInstrumentationGoImage,
		autoInstrumentationApacheHttpdImage: o.autoInstrumentationApacheHttpdImage,
		autoInstrumentationNginxImage:       o.autoInstrumentationNginxImage,
		dcgmExporterImage:                   o.dcgmExporterImage,
		neuronMonitorImage:                  o.neuronMonitorImage,
		labelsFilter:                        o.labelsFilter,
	}
}

// CollectorImage represents the flag to override the OpenTelemetry Collector container image.
func (c *Config) CollectorImage() string {
	return c.collectorImage
}

// CollectorConfigMapEntry represents the configuration file name for the collector. Immutable.
func (c *Config) CollectorConfigMapEntry() string {
	return c.collectorConfigMapEntry
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
// TargetAllocatorImage represents the flag to override the OpenTelemetry TargetAllocator container image.
func (c *Config) TargetAllocatorImage() string {
	return c.targetAllocatorImage
}

// TargetAllocatorConfigMapEntry represents the configuration file name for the TargetAllocator. Immutable.
func (c *Config) TargetAllocatorConfigMapEntry() string {
	return c.targetAllocatorConfigMapEntry
}

// Platform represents the type of the platform this operator is running.
func (c *Config) Platform() platform.Platform {
	return c.platform
}

// LabelsFilter Returns the filters converted to regex strings used to filter out unwanted labels from propagations.
func (c *Config) LabelsFilter() []string {
	return c.labelsFilter
}
