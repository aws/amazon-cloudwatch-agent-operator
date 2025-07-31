// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifestutils

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func isFilteredLabel(label string, filterLabels []string) bool {
	for _, pattern := range filterLabels {
		match, _ := regexp.MatchString(pattern, label)
		return match
	}
	return false
}

// Labels return the common labels to all objects that are part of a managed CR.
func Labels(instance metav1.ObjectMeta, name string, image string, component string, filterLabels []string) map[string]string {
	var versionLabel string
	// new map every time, so that we don't touch the instance's label
	base := map[string]string{}
	if nil != instance.Labels {
		for k, v := range instance.Labels {
			if !isFilteredLabel(k, filterLabels) {
				base[k] = v
			}
		}
	}

	for k, v := range SelectorLabels(instance, component) {
		base[k] = v
	}

	if len(image) > 0 {
		version := strings.Split(image, ":")
		for _, v := range version {
			if strings.HasSuffix(v, "@sha256") {
				versionLabel = strings.TrimSuffix(v, "@sha256")
			}
		}
		switch lenVersion := len(version); lenVersion {
		case 3:
			base["app.kubernetes.io/version"] = versionLabel
		case 2:
			base["app.kubernetes.io/version"] = naming.Truncate("%s", 63, version[len(version)-1])
		default:
			base["app.kubernetes.io/version"] = "latest"
		}
	}

	// Don't override the app name if it already exists
	if _, ok := base["app.kubernetes.io/name"]; !ok {
		base["app.kubernetes.io/name"] = name
	}
	return base
}

// SelectorLabels return the common labels to all objects that are part of a managed CR to use as selector.
// Selector labels are immutable for Deployment, StatefulSet and DaemonSet, therefore, no labels in selector should be
// expected to be modified for the lifetime of the object.
func SelectorLabels(instance metav1.ObjectMeta, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   naming.Truncate("%s.%s", 63, instance.Namespace, instance.Name),
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/component":  component,
	}
}

func SelectorLabelsForAllOperatorManaged(instance metav1.ObjectMeta) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   naming.Truncate("%s.%s", 63, instance.Namespace, instance.Name),
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
	}
}

// CreateLivenessProbe creates a standard liveness probe for health endpoints
func CreateLivenessProbe(path string, port intstr.IntOrString, probeConfig *v1alpha1.Probe) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: port,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
		TimeoutSeconds:      10,
		FailureThreshold:    5,
	}

	// Apply custom probe configuration if provided
	if probeConfig != nil {
		if probeConfig.InitialDelaySeconds != nil {
			probe.InitialDelaySeconds = *probeConfig.InitialDelaySeconds
		}
		if probeConfig.PeriodSeconds != nil {
			probe.PeriodSeconds = *probeConfig.PeriodSeconds
		}
		if probeConfig.FailureThreshold != nil {
			probe.FailureThreshold = *probeConfig.FailureThreshold
		}
		if probeConfig.SuccessThreshold != nil {
			probe.SuccessThreshold = *probeConfig.SuccessThreshold
		}
		if probeConfig.TimeoutSeconds != nil {
			probe.TimeoutSeconds = *probeConfig.TimeoutSeconds
		}
		probe.TerminationGracePeriodSeconds = probeConfig.TerminationGracePeriodSeconds
	}

	return probe
}

// CreateReadinessProbe creates a standard readiness probe for health endpoints
func CreateReadinessProbe(path string, port intstr.IntOrString, probeConfig *v1alpha1.Probe) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: port,
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      10,
		FailureThreshold:    5,
	}

	// Apply custom probe configuration if provided
	if probeConfig != nil {
		if probeConfig.InitialDelaySeconds != nil {
			probe.InitialDelaySeconds = *probeConfig.InitialDelaySeconds
		}
		if probeConfig.PeriodSeconds != nil {
			probe.PeriodSeconds = *probeConfig.PeriodSeconds
		}
		if probeConfig.FailureThreshold != nil {
			probe.FailureThreshold = *probeConfig.FailureThreshold
		}
		if probeConfig.SuccessThreshold != nil {
			probe.SuccessThreshold = *probeConfig.SuccessThreshold
		}
		if probeConfig.TimeoutSeconds != nil {
			probe.TimeoutSeconds = *probeConfig.TimeoutSeconds
		}
		probe.TerminationGracePeriodSeconds = probeConfig.TerminationGracePeriodSeconds
	}

	return probe
}
