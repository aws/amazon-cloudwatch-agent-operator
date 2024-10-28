// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package naming is for determining the names for components (containers, services, ...).
package naming

// ConfigMap builds the name for the config map used in the AmazonCloudWatchAgent containers.
func ConfigMap(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// TAConfigMap returns the name for the config map used in the TargetAllocator.
func TAConfigMap(otelcol string) string {
	return DNSName(Truncate("%s-target-allocator", 63, otelcol))
}

// PrometheusConfigMap returns the name for the prometheus config map.
func PrometheusConfigMap(otelcol string) string {
	return DNSName(Truncate("%s-prometheus-config", 63, otelcol))
}

// ConfigMapVolume returns the name to use for the config map's volume in the pod.
func ConfigMapVolume() string {
	return "otc-internal"
}

// ConfigMapExtra returns the prefix to use for the extras mounted configmaps in the pod.
func ConfigMapExtra(extraConfigMapName string) string {
	return DNSName(Truncate("configmap-%s", 63, extraConfigMapName))
}

// TAConfigMapVolume returns the name to use for the config map's volume in the TargetAllocator pod.
func TAConfigMapVolume() string {
	return "ta-internal"
}
func TASecretVolume() string {
	return "ta-secret"
}

// PrometheusConfigMapVolume returns the name to use for the prometheus config map's volume in the pod.
func PrometheusConfigMapVolume() string {
	return "prometheus-config"
}

// Container returns the name to use for the container in the pod.
func Container() string {
	return "otc-container"
}

// TAContainer returns the name to use for the container in the TargetAllocator pod.
func TAContainer() string {
	return "ta-container"
}

// Collector builds the collector (deployment/daemonset) name based on the instance.
func Collector(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// HorizontalPodAutoscaler builds the autoscaler name based on the instance.
func HorizontalPodAutoscaler(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// PodDisruptionBudget builds the pdb name based on the instance.
func PodDisruptionBudget(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// AmazonCloudWatchAgent builds the collector (deployment/daemonset) name based on the instance.
func AmazonCloudWatchAgent(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// AmazonCloudWatchAgentName builds the collector (deployment/daemonset) name based on the instance.
func AmazonCloudWatchAgentName(otelcolName string) string {
	return DNSName(Truncate("%s", 63, otelcolName))
}

// TargetAllocator returns the TargetAllocator deployment resource name.
func TargetAllocator(otelcol string) string {
	return DNSName(Truncate("%s-target-allocator", 63, otelcol))
}

// HeadlessService builds the name for the headless service based on the instance.
func HeadlessService(otelcol string) string {
	return DNSName(Truncate("%s-headless", 63, Service(otelcol)))
}

// MonitoringService builds the name for the monitoring service based on the instance.
func MonitoringService(otelcol string) string {
	return DNSName(Truncate("%s-monitoring", 63, Service(otelcol)))
}

// Service builds the service name based on the instance.
func Service(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// Ingress builds the ingress name based on the instance.
func Ingress(otelcol string) string {
	return DNSName(Truncate("%s-ingress", 63, otelcol))
}

// Route builds the route name based on the instance.
func Route(otelcol string, prefix string) string {
	return DNSName(Truncate("%s-%s-route", 63, prefix, otelcol))
}

// TAService returns the name to use for the TargetAllocator service.
func TAService() string {
	return "target-allocator-service"
}
func TAPodDestination(otelcol string) string {
	return DNSName(Truncate("%s-target-allocator", 63, otelcol))
}

// ServiceAccount builds the service account name based on the instance.
func ServiceAccount(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// ServiceMonitor builds the service Monitor name based on the instance.
func ServiceMonitor(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}

// PodMonitor builds the pod Monitor name based on the instance.
func PodMonitor(otelcol string) string {
	return DNSName(Truncate("%s", 63, otelcol))
}
