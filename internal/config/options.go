// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"
	"strings"

	"github.com/go-logr/logr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

// Option represents one specific configuration option.
type Option func(c *options)

type options struct {
	version                             version.Version
	logger                              logr.Logger
	autoInstrumentationDotNetImage      string
	autoInstrumentationGoImage          string
	autoInstrumentationJavaImage        string
	autoInstrumentationNodeJSImage      string
	autoInstrumentationPythonImage      string
	autoInstrumentationApacheHttpdImage string
	autoInstrumentationNginxImage       string
	collectorImage                      string
	collectorConfigMapEntry             string
	otelCollectorConfigMapEntry         string
	dcgmExporterImage                   string
	neuronMonitorImage                  string
	targetAllocatorImage                string
	targetAllocatorConfigMapEntry       string
	prometheusConfigMapEntry            string
	labelsFilter                        []string
}

func WithCollectorImage(s string) Option {
	return func(o *options) {
		o.collectorImage = s
	}
}
func WithCollectorConfigMapEntry(s string) Option {
	return func(o *options) {
		o.collectorConfigMapEntry = s
	}
}
func WithOtelCollectorConfigMapEntry(s string) Option {
	return func(o *options) {
		o.otelCollectorConfigMapEntry = s
	}
}
func WithTargetAllocatorConfigMapEntry(s string) Option {
	return func(o *options) {
		o.targetAllocatorConfigMapEntry = s
	}
}
func WithPrometheusConfigMapEntry(s string) Option {
	return func(o *options) {
		o.prometheusConfigMapEntry = s
	}
}
func WithLogger(logger logr.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}
func WithVersion(v version.Version) Option {
	return func(o *options) {
		o.version = v
	}
}

func WithAutoInstrumentationJavaImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationJavaImage = s
	}
}

func WithAutoInstrumentationNodeJSImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationNodeJSImage = s
	}
}

func WithAutoInstrumentationPythonImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationPythonImage = s
	}
}

func WithAutoInstrumentationDotNetImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationDotNetImage = s
	}
}

func WithAutoInstrumentationGoImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationGoImage = s
	}
}

func WithAutoInstrumentationApacheHttpdImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationApacheHttpdImage = s
	}
}

func WithAutoInstrumentationNginxImage(s string) Option {
	return func(o *options) {
		o.autoInstrumentationNginxImage = s
	}
}

func WithDcgmExporterImage(s string) Option {
	return func(o *options) {
		o.dcgmExporterImage = s
	}
}

func WithNeuronMonitorImage(s string) Option {
	return func(o *options) {
		o.neuronMonitorImage = s
	}
}

func WithTargetAllocatorImage(s string) Option {
	return func(o *options) {
		o.targetAllocatorImage = s
	}
}

func WithLabelFilters(labelFilters []string) Option {
	return func(o *options) {

		filters := []string{}
		for _, pattern := range labelFilters {
			var result strings.Builder

			for i, literal := range strings.Split(pattern, "*") {

				// Replace * with .*
				if i > 0 {
					result.WriteString(".*")
				}

				// Quote any regular expression meta characters in the
				// literal text.
				result.WriteString(regexp.QuoteMeta(literal))
			}
			filters = append(filters, result.String())
		}

		o.labelsFilter = filters
	}
}
