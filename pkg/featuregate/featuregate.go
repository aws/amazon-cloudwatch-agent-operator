// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package featuregate

import (
	"flag"

	"go.opentelemetry.io/collector/featuregate"
)

const (
	FeatureGatesFlag = "feature-gates"
)

var (
	EnableDotnetAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.dotnet",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator supports .NET auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.76.1"),
	)
	EnablePythonAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.python",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator supports Python auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.76.1"),
	)
	EnableJavaAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.java",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator supports Java auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.76.1"),
	)
	EnableNodeJSAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.nodejs",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator supports NodeJS auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.76.1"),
	)
	EnableGoAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.go",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("controls whether the operator supports Golang auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.77.0"),
	)
	EnableApacheHTTPAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.apache-httpd",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator supports Apache HTTPD auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.80.0"),
	)
	EnableNginxAutoInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.nginx",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("controls whether the operator supports Nginx auto-instrumentation"),
		featuregate.WithRegisterFromVersion("v0.86.0"),
	)

	EnableMultiInstrumentationSupport = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.multi-instrumentation",
		featuregate.StageAlpha,
		featuregate.WithRegisterFromVersion("0.86.0"),
		featuregate.WithRegisterDescription("controls whether the operator supports multi instrumentation"))

	// EnableTargetAllocatorRewrite is the feature gate that controls whether the collector's configuration should
	// automatically be rewritten when the target allocator is enabled.
	EnableTargetAllocatorRewrite = featuregate.GlobalRegistry().MustRegister(
		"operator.collector.rewritetargetallocator",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("controls whether the operator should configure the collector's targetAllocator configuration"),
		featuregate.WithRegisterFromVersion("v0.76.1"),
	)

	// PrometheusOperatorIsAvailable is the feature gate that enables features associated to the Prometheus Operator.
	PrometheusOperatorIsAvailable = featuregate.GlobalRegistry().MustRegister(
		"operator.observability.prometheus",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("enables features associated to the Prometheus Operator"),
		featuregate.WithRegisterFromVersion("v0.82.0"),
	)

	// SetGolangFlags is the feature gate that enables automatically setting GOMEMLIMIT and GOMAXPROCS for the
	// collector, bridge, and target allocator.
	SetGolangFlags = featuregate.GlobalRegistry().MustRegister(
		"operator.golang.flags",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("enables feature to set GOMEMLIMIT and GOMAXPROCS automatically"),
		featuregate.WithRegisterFromVersion("v0.100.0"),
	)

	// SkipMultiInstrumentationContainerValidation is the feature gate that controls whether the operator will skip
	// container name validation during pod mutation for multi-instrumentation. Enabling this feature allows multiple
	// instrumentations for pods without specified container name annotations. Does not prevent specification
	// annotations from being used.
	SkipMultiInstrumentationContainerValidation = featuregate.GlobalRegistry().MustRegister(
		"operator.autoinstrumentation.multi-instrumentation.skip-container-validation",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("controls whether the operator validates the container annotations when multi-instrumentation is enabled"))
)

// Flags creates a new FlagSet that represents the available featuregate flags using the supplied featuregate registry.
func Flags(reg *featuregate.Registry) *flag.FlagSet {
	flagSet := new(flag.FlagSet)
	flagSet.Var(featuregate.NewFlag(reg), FeatureGatesFlag,
		"Comma-delimited list of feature gate identifiers. Prefix with '-' to disable the feature. '+' or no prefix will enable the feature.")
	return flagSet
}
