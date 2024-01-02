// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// Labels return the common labels to all TargetAllocator objects that are part of a managed AmazonCloudWatchAgent.
func Labels(instance v1alpha1.AmazonCloudWatchAgent, name string) map[string]string {
	return manifestutils.Labels(instance.ObjectMeta, name, instance.Spec.TargetAllocator.Image, ComponentOpenTelemetryTargetAllocator, nil)
}

// SelectorLabels return the selector labels for Target Allocator Pods.
func SelectorLabels(instance v1alpha1.AmazonCloudWatchAgent) map[string]string {
	selectorLabels := manifestutils.SelectorLabels(instance.ObjectMeta, ComponentOpenTelemetryTargetAllocator)
	// TargetAllocator uses the name label as well for selection
	// This is inconsistent with the Collector, but changing is a somewhat painful breaking change
	selectorLabels["app.kubernetes.io/name"] = naming.TargetAllocator(instance.Name)
	return selectorLabels
}
