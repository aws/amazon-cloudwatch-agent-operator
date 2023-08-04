// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/open-telemetry/opentelemetry-operator/pkg/autodetect"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

// Option represents one specific configuration option.
type Option func(c *options)

type options struct {
	autoDetect                          autodetect.AutoDetect
	version                             version.Version
	logger                              logr.Logger
	autoInstrumentationDotNetImage      string
	autoInstrumentationGoImage          string
	autoInstrumentationJavaImage        string
	autoInstrumentationNodeJSImage      string
	autoInstrumentationPythonImage      string
	autoInstrumentationApacheHttpdImage string
	collectorImage                      string
	collectorConfigMapEntry             string
	targetAllocatorConfigMapEntry       string
	targetAllocatorImage                string
	operatorOpAMPBridgeImage            string
	onOpenShiftRoutesChange             changeHandler
	labelsFilter                        []string
	openshiftRoutes                     openshiftRoutesStore
	hpaVersion                          hpaVersionStore
	autoDetectFrequency                 time.Duration
}

func WithAutoDetect(a autodetect.AutoDetect) Option {
	return func(o *options) {
		o.autoDetect = a
	}
}
func WithAutoDetectFrequency(t time.Duration) Option {
	return func(o *options) {
		o.autoDetectFrequency = t
	}
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
func WithTargetAllocatorConfigMapEntry(s string) Option {
	return func(o *options) {
		o.targetAllocatorConfigMapEntry = s
	}
}
func WithLogger(logger logr.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithOnOpenShiftRoutesChangeCallback(f func() error) Option {
	return func(o *options) {
		if o.onOpenShiftRoutesChange == nil {
			o.onOpenShiftRoutesChange = newOnChange()
		}
		o.onOpenShiftRoutesChange.Register(f)
	}
}
func WithPlatform(ora autodetect.OpenShiftRoutesAvailability) Option {
	return func(o *options) {
		o.openshiftRoutes.Set(ora)
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
