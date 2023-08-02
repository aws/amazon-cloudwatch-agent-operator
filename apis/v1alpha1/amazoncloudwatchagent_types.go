package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Ingress is used to specify how CloudWatch Agent is exposed. This
// functionality is only available if one of the valid modes is set.
// Valid modes are: deployment, daemonset and statefulset.
// NOTE: If this feature is activated, all specified receivers are exposed.
// Currently this has a few limitations. Depending on the ingress controller
// there are problems with TLS and gRPC.
// SEE: https://github.com/open-telemetry/opentelemetry-operator/issues/1306.
// NOTE: As a workaround, port name and appProtocol could be specified directly
// in the CR.
// SEE: AmazonCloudWatchAgent.spec.ports[index].
type Ingress struct {
	// Type default value is: ""
	// Supported types are: ingress
	Type IngressType `json:"type,omitempty"`

	// Hostname by which the ingress proxy can be reached.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Annotations to add to ingress.
	// e.g. 'cert-manager.io/cluster-issuer: "letsencrypt"'
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLS configuration.
	// +optional
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`

	// IngressClassName is the name of an IngressClass cluster resource. Ingress
	// controller implementations use this field to know whether they should be
	// serving this Ingress resource.
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Route is an OpenShift specific section that is only considered when
	// type "route" is used.
	// +optional
	Route OpenShiftRoute `json:"route,omitempty"`
}

// OpenShiftRoute defines openshift route specific settings.
type OpenShiftRoute struct {
	// Termination indicates termination type. By default "edge" is used.
	Termination TLSRouteTerminationType `json:"termination,omitempty"`
}

// AmazonCloudWatchAgentSpec defines the desired state of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentSpec struct {
	// Resources to set on the CloudWatch Agent pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule CloudWatch Agent pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Args is the set of arguments to pass to the CloudWatch Agent binary
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// Replicas is the number of pod instances for the underlying CloudWatch Agent. Set this if your are not using autoscaling
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// PodAnnotations is the set of annotations that will be attached to
	// Collector and Target Allocator pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Mode represents how the collector should be deployed (deployment, daemonset, statefulset or sidecar)
	// +optional
	Mode Mode `json:"mode,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the collector.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the CloudWatch Agent.
	// +optional
	Image string `json:"image,omitempty"`
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +optional
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy"`

	// ImagePullPolicy indicates the pull policy to be used for retrieving the container image (Always, Never, IfNotPresent)
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Config is the raw JSON to be used as the collector's configuration. Refer to the CloudWatch Agent documentation for details.
	// +required
	Config string `json:"config,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying collector deployment(s)
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// ENV vars to set on the CloudWatch Agent's Pods. These can then in certain cases be
	// consumed in the config file for the Collector.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables on the CloudWatch Agent's Pods.
	// These can then in certain cases be consumed in the config file for the Collector.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
	// VolumeClaimTemplates will provide stable storage using PersistentVolumes. Only available when the mode=statefulset.
	// +optional
	// +listType=atomic
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// Toleration to schedule CloudWatch Agent pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Volumes represents which volumes to use in the underlying collector deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// Ingress is used to specify how CloudWatch Agent is exposed. This
	// functionality is only available if one of the valid modes is set.
	// Valid modes are: deployment, daemonset and statefulset.
	// +optional
	Ingress Ingress `json:"ingress,omitempty"`
	// HostNetwork indicates if the pod should run in the host networking namespace.
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
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

// AmazonCloudWatchAgentStatus defines the observed state of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentStatus struct {
	// Scale is the AmazonCloudWatchAgent's scale subresource status.
	// +optional
	Scale ScaleSubresourceStatus `json:"scale,omitempty"`

	// Version of the managed CloudWatch Agent (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the CloudWatch Agent.
	// +optional
	Image string `json:"image,omitempty"`

	// Messages about actions performed by the operator on this resource.
	// +optional
	// +listType=atomic
	// Deprecated: use Kubernetes events instead.
	Messages []string `json:"messages,omitempty"`

	// Replicas is currently not being set and might be removed in the next version.
	// +optional
	// Deprecated: use "AmazonCloudWatchAgent.Status.Scale.Replicas" instead.
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=otelcol;otelcols
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode",description="Deployment Mode"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="CloudWatch Agent Version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.scale.statusReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +operator-sdk:csv:customresourcedefinitions:displayName="CloudWatch Agent"
// This annotation provides a hint for OLM which resources are managed by AmazonCloudWatchAgent kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1}}

// AmazonCloudWatchAgent is the Schema for the AmazonCloudWatchAgents API.
type AmazonCloudWatchAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AmazonCloudWatchAgentSpec   `json:"spec,omitempty"`
	Status AmazonCloudWatchAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AmazonCloudWatchAgentList contains a list of AmazonCloudWatchAgent.
type AmazonCloudWatchAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AmazonCloudWatchAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AmazonCloudWatchAgent{}, &AmazonCloudWatchAgentList{})
}
