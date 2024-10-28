// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	EnvOTELServiceName          = "OTEL_SERVICE_NAME"
	EnvOTELExporterOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvOTELResourceAttrs        = "OTEL_RESOURCE_ATTRIBUTES"
	EnvOTELPropagators          = "OTEL_PROPAGATORS"
	EnvOTELTracesSampler        = "OTEL_TRACES_SAMPLER"
	EnvOTELTracesSamplerArg     = "OTEL_TRACES_SAMPLER_ARG"

	InstrumentationPrefix                           = "instrumentation.opentelemetry.io/"
	AnnotationDefaultAutoInstrumentationJava        = InstrumentationPrefix + "default-auto-instrumentation-java-image"
	AnnotationDefaultAutoInstrumentationNodeJS      = InstrumentationPrefix + "default-auto-instrumentation-nodejs-image"
	AnnotationDefaultAutoInstrumentationPython      = InstrumentationPrefix + "default-auto-instrumentation-python-image"
	AnnotationDefaultAutoInstrumentationDotNet      = InstrumentationPrefix + "default-auto-instrumentation-dotnet-image"
	AnnotationDefaultAutoInstrumentationGo          = InstrumentationPrefix + "default-auto-instrumentation-go-image"
	AnnotationDefaultAutoInstrumentationApacheHttpd = InstrumentationPrefix + "default-auto-instrumentation-apache-httpd-image"
	AnnotationDefaultAutoInstrumentationNginx       = InstrumentationPrefix + "default-auto-instrumentation-nginx-image"

	EnvPodName  = "OTEL_RESOURCE_ATTRIBUTES_POD_NAME"
	EnvPodUID   = "OTEL_RESOURCE_ATTRIBUTES_POD_UID"
	EnvNodeName = "OTEL_RESOURCE_ATTRIBUTES_NODE_NAME"

	AWSEntityPrefix       = "com.amazonaws.cloudwatch.entity.internal."
	ServiceNameSource     = AWSEntityPrefix + "service.name.source"
	SourceInstrumentation = "Instrumentation"
	SourceK8sWorkload     = "K8sWorkload"
)
