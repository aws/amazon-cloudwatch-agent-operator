// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package version contains the operator's version, as well as versions of underlying components.
package version

import (
	"fmt"
	"runtime"
)

var (
	version                 string
	buildDate               string
	agent                   string
	autoInstrumentationJava string
)

// Version holds this Operator's version as well as the version of some of the components it uses.
type Version struct {
	Operator                string `json:"amazon-cloudwatch-agent-operator"`
	BuildDate               string `json:"build-date"`
	AmazonCloudWatchAgent   string `json:"amazon-cloudwatch-agent-version"`
	Go                      string `json:"go-version"`
	AutoInstrumentationJava string `json:"auto-instrumentation-java"`
}

// Get returns the Version object with the relevant information.
func Get() Version {
	return Version{
		Operator:                version,
		BuildDate:               buildDate,
		AmazonCloudWatchAgent:   AmazonCloudWatchAgent(),
		Go:                      runtime.Version(),
		AutoInstrumentationJava: AutoInstrumentationJava(),
	}
}

func (v Version) String() string {
	return fmt.Sprintf(
		"Version(Operator='%v', BuildDate='%v', AmazonCloudWatchAgent='%v', Go='%v', AutoInstrumentationJava='%v')",
		v.Operator,
		v.BuildDate,
		v.AmazonCloudWatchAgent,
		v.Go,
		v.AutoInstrumentationJava,
	)
}

// AmazonCloudWatchAgent returns the default AmazonCloudWatchAgent to use when no versions are specified via CLI or configuration.
func AmazonCloudWatchAgent() string {
	if len(agent) > 0 {
		// this should always be set, as it's specified during the build
		return agent
	}

	// fallback value, useful for tests
	return "0.0.0"
}

func AutoInstrumentationJava() string {
	if len(autoInstrumentationJava) > 0 {
		return autoInstrumentationJava
	}
	return "0.0.0"
}
