// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NeuronMonitorSpec defines the desired state of NeuronMonitor.
type NeuronMonitorSpec struct {
	// Resources to set on the Neuron Monitor Exporter pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule Neuron Monitor Exporter pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// SecurityContext configures the container security context for
	// the amazon-cloudwatch-agent container.
	//
	// In deployment, daemonset, or statefulset mode, this controls
	// the security context settings for the primary application
	// container.
	//
	// In sidecar mode, this controls the security context for the
	// injected sidecar container.
	//
	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
	// Entrypoint array. Not executed within a shell.
	// The container image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	Command []string `json:"command,omitempty"`
	// Args is the set of arguments to pass to the Neuron Monitor Exporter binary
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the collector.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the Neuron Monitor Exporter.
	// +optional
	Image string `json:"image,omitempty"`
	// MonitorConfig is the raw Json to be used as monitor configuration.
	// +required
	MonitorConfig string `json:"monitorConfig,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// ENV vars to set on the Neuron Monitor Exporter Pods. These can then in certain cases be
	// consumed in the config file for the Collector.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// Toleration to schedule Neuron Monitor Exporter pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Volumes represents which volumes to use in the underlying collector deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying collector deployment(s)
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// If specified, indicates the pod's scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
}

// NeuronMonitorStatus defines the observed state of NeuronMonitor.
type NeuronMonitorStatus struct {
	// Scale is the NeuronMonitor's scale subresource status.
	// +optional
	Scale ScaleSubresourceStatus `json:"scale,omitempty"`

	// Version of the managed Neuron Monitor Exporter (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the Neuron Monitor Exporter.
	// +optional
	Image string `json:"image,omitempty"`

	// Messages about actions performed by the operator on this resource.
	// +optional
	// +listType=atomic
	// Deprecated: use Kubernetes events instead.
	Messages []string `json:"messages,omitempty"`

	// Replicas is currently not being set and might be removed in the next version.
	// +optional
	// Deprecated: use "NeuronMonitor.Status.Scale.Replicas" instead.
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=neuronexp;neuronexps
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Neuron Monitor exporter Version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.scale.statusReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Management",type="string",JSONPath=".spec.managementState",description="Management State"
// +operator-sdk:csv:customresourcedefinitions:displayName="Neuron Monitor"
// This annotation provides a hint for OLM which resources are managed by NeuronMonitor kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1}}

// NeuronMonitor is the Schema for the NeuronMonitor API.
type NeuronMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NeuronMonitorSpec   `json:"spec,omitempty"`
	Status NeuronMonitorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NeuronMonitorList contains a list of NeuronMonitor.
type NeuronMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NeuronMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NeuronMonitor{}, &NeuronMonitorList{})
}
