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
)

// Config holds the static configuration for this operator.
type Config struct {
	logger                              logr.Logger
	autoInstrumentationPythonImage      string
	collectorImage                      string
	collectorConfigMapEntry             string
	enableMultiInstrumentation          bool
	enableApacheHttpdInstrumentation    bool
	enableDotNetInstrumentation         bool
	enableGoInstrumentation             bool
	enableNginxInstrumentation          bool
	enablePythonInstrumentation         bool
	enableNodeJSInstrumentation         bool
	enableJavaInstrumentation           bool
	autoInstrumentationDotNetImage      string
	autoInstrumentationGoImage          string
	autoInstrumentationApacheHttpdImage string
	autoInstrumentationNginxImage       string
	autoInstrumentationNodeJSImage      string
	autoInstrumentationJavaImage        string
	dcgmExporterImage                   string
	neuronMonitorImage                  string
	labelsFilter                        []string
	annotationsFilter                   []string
}

// New constructs a new configuration based on the given options.
func New(opts ...Option) Config {
	// initialize with the default values
	o := options{
		collectorConfigMapEntry:   defaultCollectorConfigMapEntry,
		logger:                    logf.Log.WithName("config"),
		version:                   version.Get(),
		enableJavaInstrumentation: true,
		annotationsFilter:         []string{"kubectl.kubernetes.io/last-applied-configuration"},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return Config{
		collectorImage:                      o.collectorImage,
		collectorConfigMapEntry:             o.collectorConfigMapEntry,
		enableMultiInstrumentation:          o.enableMultiInstrumentation,
		enableApacheHttpdInstrumentation:    o.enableApacheHttpdInstrumentation,
		enableDotNetInstrumentation:         o.enableDotNetInstrumentation,
		enableGoInstrumentation:             o.enableGoInstrumentation,
		enableNginxInstrumentation:          o.enableNginxInstrumentation,
		enablePythonInstrumentation:         o.enablePythonInstrumentation,
		enableNodeJSInstrumentation:         o.enableNodeJSInstrumentation,
		enableJavaInstrumentation:           o.enableJavaInstrumentation,
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
		labelsFilter:                        o.labelsFilter,
		annotationsFilter:                   o.annotationsFilter,
	}
}

// CollectorImage represents the flag to override the OpenTelemetry Collector container image.
func (c *Config) CollectorImage() string {
	return c.collectorImage
}

// EnableMultiInstrumentation is true when the operator supports multi instrumentation.
func (c *Config) EnableMultiInstrumentation() bool {
	return c.enableMultiInstrumentation
}

// EnableApacheHttpdAutoInstrumentation is true when the operator supports ApacheHttpd auto instrumentation.
func (c *Config) EnableApacheHttpdAutoInstrumentation() bool {
	return c.enableApacheHttpdInstrumentation
}

// EnableDotNetAutoInstrumentation is true when the operator supports dotnet auto instrumentation.
func (c *Config) EnableDotNetAutoInstrumentation() bool {
	return c.enableDotNetInstrumentation
}

// EnableGoAutoInstrumentation is true when the operator supports Go auto instrumentation.
func (c *Config) EnableGoAutoInstrumentation() bool {
	return c.enableGoInstrumentation
}

// EnableNginxAutoInstrumentation is true when the operator supports nginx auto instrumentation.
func (c *Config) EnableNginxAutoInstrumentation() bool {
	return c.enableNginxInstrumentation
}

// EnableJavaAutoInstrumentation is true when the operator supports nginx auto instrumentation.
func (c *Config) EnableJavaAutoInstrumentation() bool {
	return c.enableJavaInstrumentation
}

// EnablePythonAutoInstrumentation is true when the operator supports dotnet auto instrumentation.
func (c *Config) EnablePythonAutoInstrumentation() bool {
	return c.enablePythonInstrumentation
}

// EnableNodeJSAutoInstrumentation is true when the operator supports dotnet auto instrumentation.
func (c *Config) EnableNodeJSAutoInstrumentation() bool {
	return c.enableNodeJSInstrumentation
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
}

// LabelsFilter Returns the filters converted to regex strings used to filter out unwanted labels from propagations.
func (c *Config) LabelsFilter() []string {
	return c.labelsFilter
}

// AnnotationsFilter Returns the filters converted to regex strings used to filter out unwanted labels from propagations.
func (c *Config) AnnotationsFilter() []string {
	return c.annotationsFilter
}
