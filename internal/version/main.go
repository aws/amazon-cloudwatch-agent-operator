// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package version contains the operator's version, as well as versions of underlying components.
package version

import (
	"fmt"
	"runtime"
)

var (
	version                        string
	buildDate                      string
	otelCol                        string
	autoInstrumentationJava        string
	autoInstrumentationNodeJS      string
	autoInstrumentationPython      string
	autoInstrumentationDotNet      string
	autoInstrumentationApacheHttpd string
	autoInstrumentationNginx       string
	autoInstrumentationGo          string
	dcgmExporter                   string
)

// Version holds this Operator's version as well as the version of some of the components it uses.
type Version struct {
	Operator                       string `json:"amazon-cloudwatch-agent-operator"`
	BuildDate                      string `json:"build-date"`
	AmazonCloudWatchAgent          string `json:"amazon-cloudwatch-agent-version"`
	Go                             string `json:"go-version"`
	AutoInstrumentationJava        string `json:"auto-instrumentation-java"`
	AutoInstrumentationNodeJS      string `json:"auto-instrumentation-nodejs"`
	AutoInstrumentationPython      string `json:"auto-instrumentation-python"`
	AutoInstrumentationDotNet      string `json:"auto-instrumentation-dotnet"`
	AutoInstrumentationGo          string `json:"auto-instrumentation-go"`
	AutoInstrumentationApacheHttpd string `json:"auto-instrumentation-apache-httpd"`
	AutoInstrumentationNginx       string `json:"auto-instrumentation-nginx"`
	DcgmExporter                   string `json:"dcgm-exporter-version"`
}

// Get returns the Version object with the relevant information.
func Get() Version {
	return Version{
		Operator:                       version,
		BuildDate:                      buildDate,
		AmazonCloudWatchAgent:          AmazonCloudWatchAgent(),
		Go:                             runtime.Version(),
		AutoInstrumentationJava:        AutoInstrumentationJava(),
		AutoInstrumentationNodeJS:      AutoInstrumentationNodeJS(),
		AutoInstrumentationPython:      AutoInstrumentationPython(),
		AutoInstrumentationDotNet:      AutoInstrumentationDotNet(),
		AutoInstrumentationGo:          AutoInstrumentationGo(),
		AutoInstrumentationApacheHttpd: AutoInstrumentationApacheHttpd(),
		AutoInstrumentationNginx:       AutoInstrumentationNginx(),
		DcgmExporter:                   DcgmExporter(),
	}
}

func (v Version) String() string {
	return fmt.Sprintf(
		"Version(Operator='%v', BuildDate='%v', AmazonCloudWatchAgent='%v', Go='%v', AutoInstrumentationJava='%v', AutoInstrumentationNodeJS='%v', AutoInstrumentationPython='%v', AutoInstrumentationDotNet='%v', AutoInstrumentationGo='%v', AutoInstrumentationApacheHttpd='%v', AutoInstrumentationNginx='%v', DcgmExporter='%v')",
		v.Operator,
		v.BuildDate,
		v.AmazonCloudWatchAgent,
		v.Go,
		v.AutoInstrumentationJava,
		v.AutoInstrumentationNodeJS,
		v.AutoInstrumentationPython,
		v.AutoInstrumentationDotNet,
		v.AutoInstrumentationGo,
		v.AutoInstrumentationApacheHttpd,
		v.AutoInstrumentationNginx,
		v.DcgmExporter,
	)
}

// AmazonCloudWatchAgent returns the default AmazonCloudWatchAgent to use when no versions are specified via CLI or configuration.
func AmazonCloudWatchAgent() string {
	if len(otelCol) > 0 {
		// this should always be set, as it's specified during the build
		return otelCol
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

func AutoInstrumentationNodeJS() string {
	if len(autoInstrumentationNodeJS) > 0 {
		return autoInstrumentationNodeJS
	}
	return "0.0.0"
}

func AutoInstrumentationPython() string {
	if len(autoInstrumentationPython) > 0 {
		return autoInstrumentationPython
	}
	return "0.0.0"
}

func AutoInstrumentationDotNet() string {
	if len(autoInstrumentationDotNet) > 0 {
		return autoInstrumentationDotNet
	}
	return "0.0.0"
}

func AutoInstrumentationApacheHttpd() string {
	if len(autoInstrumentationApacheHttpd) > 0 {
		return autoInstrumentationApacheHttpd
	}
	return "0.0.0"
}

func AutoInstrumentationNginx() string {
	if len(autoInstrumentationNginx) > 0 {
		return autoInstrumentationNginx
	}
	return "0.0.0"
}

func AutoInstrumentationGo() string {
	if len(autoInstrumentationGo) > 0 {
		return autoInstrumentationGo
	}
	return "0.0.0"
}

func DcgmExporter() string {
	if len(dcgmExporter) > 0 {
		// this should always be set, as it's specified during the build
		return dcgmExporter
	}
	return "0.0.0"
}
