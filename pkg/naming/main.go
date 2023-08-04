// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package naming is for determining the names for components (containers, services, ...).
package naming

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// ConfigMap builds the name for the config map used in the AmazonCloudWatchAgent containers.
func ConfigMap(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s", 63, agent.Name))
}

// ConfigMapVolume returns the name to use for the config map's volume in the pod.
func ConfigMapVolume() string {
	return "cwaagentconfig"
}

// Container returns the name to use for the container in the pod.
func Container() string {
	return "cwa-container"
}

// Agent builds the agent (deployment/daemonset) name based on the instance.
func Agent(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s", 63, agent.Name))
}

// AmazonCloudWatchAgent builds the agent (deployment/daemonset) name based on the instance.
func AmazonCloudWatchAgent(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s", 63, agent.Name))
}

// AmazonCloudWatchAgentName builds the agent (deployment/daemonset) name based on the instance.
func AmazonCloudWatchAgentName(agentName string) string {
	return DNSName(Truncate("%s", 63, agentName))
}

// HeadlessService builds the name for the headless service based on the instance.
func HeadlessService(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s-headless", 63, Service(agent)))
}

// MonitoringService builds the name for the monitoring service based on the instance.
func MonitoringService(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s-monitoring", 63, Service(agent)))
}

// Service builds the service name based on the instance.
func Service(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s", 63, agent.Name))
}

// Ingress builds the ingress name based on the instance.
func Ingress(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s-ingress", 63, agent.Name))
}

// Route builds the route name based on the instance.
func Route(agent v1alpha1.AmazonCloudWatchAgent, prefix string) string {
	return DNSName(Truncate("%s-%s-route", 63, prefix, agent.Name))
}

// ServiceAccount builds the service account name based on the instance.
func ServiceAccount(agent v1alpha1.AmazonCloudWatchAgent) string {
	return DNSName(Truncate("%s", 63, "cloudwatch-agent"))
}
