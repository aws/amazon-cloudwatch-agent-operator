// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&AmazonCloudWatchAgent{}, &AmazonCloudWatchAgentList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=otelcol;otelcols
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode",description="Deployment Mode"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="CloudWatch Agent Version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.scale.statusReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Management",type="string",JSONPath=".spec.managementState",description="Management State"
// +operator-sdk:csv:customresourcedefinitions:displayName="Amazon CloudWatch Agent"
// This annotation provides a hint for OLM which resources are managed by AmazonCloudWatchAgent kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1},{Ingress,networking/v1}}

// AmazonCloudWatchAgent is the Schema for the amazoncloudwatchagents API.
type AmazonCloudWatchAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AmazonCloudWatchAgentSpec   `json:"spec,omitempty"`
	Status AmazonCloudWatchAgentStatus `json:"status,omitempty"`
}

// Hub exists to allow for conversion.
func (*AmazonCloudWatchAgent) Hub() {}

//+kubebuilder:object:root=true

// AmazonCloudWatchAgentList contains a list of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AmazonCloudWatchAgent `json:"items"`
}

// AmazonCloudWatchAgentStatus defines the observed state of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentStatus struct {
	// Scale is the AmazonCloudWatchAgent's scale subresource status.
	// +optional
	Scale ScaleSubresourceStatus `json:"scale,omitempty"`

	// Version of the managed AmazonCloudWatchAgent (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the AmazonCloudWatchAgent.
	// +optional
	Image string `json:"image,omitempty"`
}

// AmazonCloudWatchAgentSpec defines the desired state of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentSpec struct {
	// AmazonCloudWatchAgentCommonFields are fields that are on all AmazonCloudWatchAgent CRD workloads.
	AmazonCloudWatchAgentCommonFields `json:",inline"`
	// StatefulSetCommonFields are fields that are on all AmazonCloudWatchAgent CRD workloads.
	StatefulSetCommonFields `json:",inline"`
	// Autoscaler specifies the pod autoscaling configuration to use
	// for the workload.
	// +optional
	Autoscaler *AutoscalerSpec `json:"autoscaler,omitempty"`
	// Mode represents how the collector should be deployed (deployment, daemonset, statefulset or sidecar)
	// +optional
	Mode Mode `json:"mode,omitempty"`
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +optional
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy"`
	// Config is the raw JSON to be used as the collector's configuration. Refer to the AmazonCloudWatchAgent documentation for details.
	// +required
	Config string `json:"config,omitempty"`
	// ConfigVersions defines the number versions to keep for the collector config. Each config version is stored in a separate ConfigMap.
	// Defaults to 3. The minimum value is 1.
	// +optional
	// +kubebuilder:default:=3
	// +kubebuilder:validation:Minimum:=1
	ConfigVersions int `json:"configVersions,omitempty"`
	// Ingress is used to specify how AmazonCloudWatchAgent is exposed. This
	// functionality is only available if one of the valid modes is set.
	// Valid modes are: deployment, daemonset and statefulset.
	// +optional
	Ingress Ingress `json:"ingress,omitempty"`
	// Liveness config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// Readiness config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`

	// ObservabilitySpec defines how telemetry data gets handled.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// ConfigMaps is a list of ConfigMaps in the same namespace as the AmazonCloudWatchAgent
	// object, which shall be mounted into the Collector Pods.
	// Each ConfigMap will be added to the Collector's Deployments as a volume named `configmap-<configmap-name>`.
	ConfigMaps []ConfigMapsSpec `json:"configmaps,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing DaemonSet pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec
	// This is only applicable to Daemonset mode.
	// +optional
	DaemonSetUpdateStrategy appsv1.DaemonSetUpdateStrategy `json:"daemonSetUpdateStrategy,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing Deployment pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#DeploymentSpec
	// This is only applicable to Deployment mode.
	// +optional
	DeploymentUpdateStrategy appsv1.DeploymentStrategy `json:"deploymentUpdateStrategy,omitempty"`
}

// Probe defines the OpenTelemetry's pod probe config.
type Probe struct {
	// Number of seconds after the container has started before liveness probes are initiated.
	// Defaults to 0 seconds. Minimum value is 0.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// ObservabilitySpec defines how telemetry data gets handled.
type ObservabilitySpec struct {
	// Metrics defines the metrics configuration for operands.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Config"
	Metrics MetricsConfigSpec `json:"metrics,omitempty"`
}

// MetricsConfigSpec defines a metrics config.
type MetricsConfigSpec struct {
	// EnableMetrics specifies if ServiceMonitor or PodMonitor(for sidecar mode) should be created for the service managed by the OpenTelemetry Operator.
	// The operator.observability.prometheus feature gate must be enabled to use this feature.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Create ServiceMonitors for AmazonCloudWatchAgent"
	EnableMetrics bool `json:"enableMetrics,omitempty"`
	// DisablePrometheusAnnotations controls the automatic addition of default Prometheus annotations
	// ('prometheus.io/scrape', 'prometheus.io/port', and 'prometheus.io/path')
	//
	// +optional
	// +kubebuilder:validation:Optional
	DisablePrometheusAnnotations bool `json:"disablePrometheusAnnotations,omitempty"`
}

// ScaleSubresourceStatus defines the observed state of the AmazonCloudWatchAgent's
// scale subresource.
type ScaleSubresourceStatus struct {
	// The selector used to match the AmazonCloudWatchAgent's
	// deployment or statefulSet pods.
	// +optional
	Selector string `json:"selector,omitempty"`

	// The total number non-terminated pods targeted by this
	// AmazonCloudWatchAgent's deployment or statefulSet.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// StatusReplicas is the number of pods targeted by this AmazonCloudWatchAgent's with a Ready Condition /
	// Total number of non-terminated pods targeted by this AmazonCloudWatchAgent's (their labels match the selector).
	// Deployment, Daemonset, StatefulSet.
	// +optional
	StatusReplicas string `json:"statusReplicas,omitempty"`
}

type ConfigMapsSpec struct {
	// Configmap defines name and path where the configMaps should be mounted.
	Name      string `json:"name"`
	MountPath string `json:"mountpath"`
}
