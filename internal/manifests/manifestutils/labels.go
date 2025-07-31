// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifestutils

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// ExtractVersionFromImage extracts the version tag from a container image string
func ExtractVersionFromImage(image string) string {
	if image == "" {
		return ""
	}

	// Split by ':' to get the tag part
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return ""
	}

	// Return the tag (version) part
	return parts[len(parts)-1]
}

// EmitHealthEvents emits health events based on pod readiness status
func EmitHealthEvents(recorder record.EventRecorder, obj client.Object, componentName string, readyReplicas, totalReplicas int32, creationTime time.Time, gracePeriod time.Duration) {
	if totalReplicas > 0 {
		if readyReplicas == totalReplicas {
			recorder.Event(obj, corev1.EventTypeNormal, "ComponentHealthy",
				fmt.Sprintf("%s is healthy: %d/%d pods ready", componentName, readyReplicas, totalReplicas))
		} else if readyReplicas == 0 {
			if time.Since(creationTime) >= gracePeriod {
				recorder.Event(obj, corev1.EventTypeWarning, "ComponentUnhealthy",
					fmt.Sprintf("%s is unhealthy: %d/%d pods ready", componentName, readyReplicas, totalReplicas))
			}
		} else {
			recorder.Event(obj, corev1.EventTypeWarning, "ComponentPartiallyHealthy",
				fmt.Sprintf("%s is partially healthy: %d/%d pods ready", componentName, readyReplicas, totalReplicas))
		}
	}
}
