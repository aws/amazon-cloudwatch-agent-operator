// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstrumentationSpec defines the desired state of OpenTelemetry SDK and instrumentation.
type InstrumentationSpec struct {
	// Exporter defines exporter configuration.
	// +optional
	Exporter `json:"exporter,omitempty"`

	// Resource defines the configuration for the resource attributes, as defined by the OpenTelemetry specification.
	// +optional
	Resource Resource `json:"resource,omitempty"`

	// Propagators defines inter-process context propagation configuration.
	// Values in this list will be set in the OTEL_PROPAGATORS env var.
	// Enum=tracecontext;baggage;b3;b3multi;jaeger;xray;ottrace;none
	// +optional
	Propagators []Propagator `json:"propagators,omitempty"`

	// Sampler defines sampling configuration.
	// +optional
	Sampler `json:"sampler,omitempty"`

	// Env defines common env vars. There are four layers for env vars' definitions and
	// the precedence order is: `original container env vars` > `language specific env vars` > `common env vars` > `instrument spec configs' vars`.
	// If the former var had been defined, then the other vars would be ignored.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Java defines configuration for java auto-instrumentation.
	// +optional
	Java Java `json:"java,omitempty"`
}

// Resource defines the configuration for the resource attributes, as defined by the OpenTelemetry specification.
// See also: https://github.com/open-telemetry/opentelemetry-specification/blob/v1.8.0/specification/overview.md#resources
type Resource struct {
	// Attributes defines attributes that are added to the resource.
	// For example environment: dev
	// +optional
	Attributes map[string]string `json:"resourceAttributes,omitempty"`

	// AddK8sUIDAttributes defines whether K8s UID attributes should be collected (e.g. k8s.deployment.uid).
	// +optional
	AddK8sUIDAttributes bool `json:"addK8sUIDAttributes,omitempty"`
}

// Exporter defines OTLP exporter configuration.
type Exporter struct {
	// Endpoint is address of the collector with OTLP endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// Sampler defines sampling configuration.
type Sampler struct {
	// Type defines sampler type.
	// The value will be set in the OTEL_TRACES_SAMPLER env var.
	// The value can be for instance parentbased_always_on, parentbased_always_off, parentbased_traceidratio...
	// +optional
	Type SamplerType `json:"type,omitempty"`

	// Argument defines sampler argument.
	// The value depends on the sampler type.
	// For instance for parentbased_traceidratio sampler type it is a number in range [0..1] e.g. 0.25.
	// The value will be set in the OTEL_TRACES_SAMPLER_ARG env var.
	// +optional
	Argument string `json:"argument,omitempty"`
}

// Java defines Java SDK and instrumentation configuration.
type Java struct {
	// Image is a container image with javaagent auto-instrumentation JAR.
	// +optional
	Image string `json:"image,omitempty"`

	// Env defines java specific env vars. There are four layers for env vars' definitions and
	// the precedence order is: `original container env vars` > `language specific env vars` > `common env vars` > `instrument spec configs' vars`.
	// If the former var had been defined, then the other vars would be ignored.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources describes the compute resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// InstrumentationStatus defines status of the instrumentation.
type InstrumentationStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=otelinst;otelinsts
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.exporter.endpoint"
// +kubebuilder:printcolumn:name="Sampler",type="string",JSONPath=".spec.sampler.type"
// +kubebuilder:printcolumn:name="Sampler Arg",type="string",JSONPath=".spec.sampler.argument"
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenTelemetry Instrumentation"
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1}}

// Instrumentation is the spec for OpenTelemetry instrumentation.
type Instrumentation struct {
	Status            InstrumentationStatus `json:"status,omitempty"`
	metav1.TypeMeta   `json:",inline"`
	Spec              InstrumentationSpec `json:"spec,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +kubebuilder:object:root=true

// InstrumentationList contains a list of Instrumentation.
type InstrumentationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instrumentation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instrumentation{}, &InstrumentationList{})
}
