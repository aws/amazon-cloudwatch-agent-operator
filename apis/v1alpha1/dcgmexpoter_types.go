// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DcgmExporterSpec defines the desired state of DcgmExporter.
type DcgmExporterSpec struct {
	// Resources to set on the DCGM Exporter pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule DCGM Exporter pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Args is the set of arguments to pass to the DCGM Exporter binary
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the collector.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the DCGM Exporter.
	// +optional
	Image string `json:"image,omitempty"`
	// MetricsConfig is the raw CSV to be used as metric configuration.
	// +required
	MetricsConfig string `json:"metricsConfig,omitempty"`
	// TlsConfig is the raw YAML to be used as the exporter TLS configuration.
	// +optional
	TlsConfig string `json:"tlsConfig,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// ENV vars to set on the DCGM Exporter Pods. These can then in certain cases be
	// consumed in the config file for the Collector.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// Toleration to schedule DCGM Exporter pods.
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
	// PodAnnotations is the set of annotations that will be attached to DCGM Exporter pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// PodLabels is the set of labels that will be attached to DCGM Exporter pods.
	// Operator-managed labels cannot be overridden and are silently ignored if
	// provided here. The reserved set includes every `app.kubernetes.io/*` key
	// the operator writes: managed-by, instance, part-of, component, name, and
	// version.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// TopologySpreadConstraints embedded Kubernetes pod configuration option,
	// controls how DCGM Exporter pods are spread across your cluster among failure-domains
	// such as regions, zones, nodes, and other user-defined topology domains.
	// See https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// If specified, indicates the pod's priority. If not specified, the pod priority
	// will be default or zero if there is no default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// PodDisruptionBudget specifies the pod disruption budget configuration to use
	// for the DcgmExporter workload.
	//
	// Note: DcgmExporter runs as a DaemonSet. Kubernetes `kubectl drain` skips
	// DaemonSet-managed pods, and the eviction API does not gate DaemonSet pods
	// the same way it does Deployment/StatefulSet pods. A PDB here is emitted
	// for parity with other configuration surfaces (e.g. EKS addon defaults)
	// but will not protect DaemonSet pods from node drains.
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// In deployment, daemonset, or statefulset mode, this controls
	// the security context settings for the primary application
	// container.
	//
	// In sidecar mode, this controls the security context for the
	// injected sidecar container.
	//
	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
}

// DcgmExporterStatus defines the observed state of DcgmExporter.
type DcgmExporterStatus struct {
	// Scale is the DcgmExporter's scale subresource status.
	// +optional
	Scale ScaleSubresourceStatus `json:"scale,omitempty"`

	// Version of the managed DCGM Exporter (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the DCGM Exporter.
	// +optional
	Image string `json:"image,omitempty"`

	// Messages about actions performed by the operator on this resource.
	// +optional
	// +listType=atomic
	// Deprecated: use Kubernetes events instead.
	Messages []string `json:"messages,omitempty"`

	// Replicas is currently not being set and might be removed in the next version.
	// +optional
	// Deprecated: use "DcgmExporter.Status.Scale.Replicas" instead.
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=dcgmexp;dcgmexps
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="DCGM exporter Version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.scale.statusReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Management",type="string",JSONPath=".spec.managementState",description="Management State"
// +operator-sdk:csv:customresourcedefinitions:displayName="DCGM Exporter"
// This annotation provides a hint for OLM which resources are managed by DcgmExporter kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1}}

// DcgmExporter is the Schema for the DcgmExporters API.
type DcgmExporter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DcgmExporterSpec   `json:"spec,omitempty"`
	Status DcgmExporterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DcgmExporterList contains a list of DcgmExporter.
type DcgmExporterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DcgmExporter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DcgmExporter{}, &DcgmExporterList{})
}
