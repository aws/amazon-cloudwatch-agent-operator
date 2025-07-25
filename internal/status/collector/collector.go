// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

func extractVersionFromImage(image string) string {
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

func UpdateCollectorStatus(ctx context.Context, cli client.Client, changed *v1alpha1.AmazonCloudWatchAgent, recorder record.EventRecorder) error {
	if changed.Status.Version == "" {
		// a version is not set, otherwise let the upgrade mechanism take care of it!
		changed.Status.Version = version.AmazonCloudWatchAgent()
	}
	mode := changed.Spec.Mode
	if mode != v1alpha1.ModeDeployment && mode != v1alpha1.ModeStatefulSet && mode != v1alpha1.ModeDaemonSet {
		changed.Status.Scale.Replicas = 0
		changed.Status.Scale.Selector = ""
		return nil
	}

	name := naming.Collector(changed.Name)

	// Set the scale selector
	labels := manifestutils.Labels(changed.ObjectMeta, name, changed.Spec.Image, collector.ComponentAmazonCloudWatchAgent, []string{})
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return fmt.Errorf("failed to get selector for labelSelector: %w", err)
	}
	changed.Status.Scale.Selector = selector.String()

	// Set the scale replicas
	objKey := client.ObjectKey{
		Namespace: changed.GetNamespace(),
		Name:      naming.Collector(changed.Name),
	}

	var replicas int32
	var readyReplicas int32
	var statusReplicas string
	var statusImage string

	switch mode { // nolint:exhaustive
	case v1alpha1.ModeDeployment:
		obj := &appsv1.Deployment{}
		if err := cli.Get(ctx, objKey, obj); err != nil {
			return fmt.Errorf("failed to get deployment status.replicas: %w", err)
		}
		replicas = obj.Status.Replicas
		readyReplicas = obj.Status.ReadyReplicas
		statusReplicas = strconv.Itoa(int(readyReplicas)) + "/" + strconv.Itoa(int(replicas))
		statusImage = obj.Spec.Template.Spec.Containers[0].Image

	case v1alpha1.ModeStatefulSet:
		obj := &appsv1.StatefulSet{}
		if err := cli.Get(ctx, objKey, obj); err != nil {
			return fmt.Errorf("failed to get statefulSet status.replicas: %w", err)
		}
		replicas = obj.Status.Replicas
		readyReplicas = obj.Status.ReadyReplicas
		statusReplicas = strconv.Itoa(int(readyReplicas)) + "/" + strconv.Itoa(int(replicas))
		statusImage = obj.Spec.Template.Spec.Containers[0].Image

	case v1alpha1.ModeDaemonSet:
		obj := &appsv1.DaemonSet{}
		if err := cli.Get(ctx, objKey, obj); err != nil {
			return fmt.Errorf("failed to get daemonSet status.replicas: %w", err)
		}
		// For DaemonSets, use different status fields
		replicas = obj.Status.DesiredNumberScheduled
		readyReplicas = obj.Status.NumberReady
		statusReplicas = strconv.Itoa(int(readyReplicas)) + "/" + strconv.Itoa(int(replicas))
		statusImage = obj.Spec.Template.Spec.Containers[0].Image
	}
	changed.Status.Scale.Replicas = replicas
	changed.Status.Image = statusImage
	changed.Status.Scale.StatusReplicas = statusReplicas

	// Extract and set version from image tag if not already set or is default
	if statusImage != "" && (changed.Status.Version == "" || changed.Status.Version == "0.0.0") {
		if imageVersion := extractVersionFromImage(statusImage); imageVersion != "" {
			changed.Status.Version = imageVersion
		}
	}

	// Emit health events based on pod readiness (for all modes including daemonset)
	if mode == v1alpha1.ModeDeployment || mode == v1alpha1.ModeStatefulSet || mode == v1alpha1.ModeDaemonSet {
		if replicas > 0 {
			if readyReplicas == replicas {
				// All pods are ready - emit Normal event
				recorder.Event(changed, "Normal", "ComponentHealthy",
					fmt.Sprintf("CloudWatch Agent is healthy: %d/%d pods ready", readyReplicas, replicas))
			} else if readyReplicas == 0 {
				// No pods are ready - emit Warning event
				recorder.Event(changed, "Warning", "ComponentUnhealthy",
					fmt.Sprintf("CloudWatch Agent is unhealthy: %d/%d pods ready", readyReplicas, replicas))
			} else {
				// Some pods are ready - emit Warning event
				recorder.Event(changed, "Warning", "ComponentPartiallyHealthy",
					fmt.Sprintf("CloudWatch Agent is partially healthy: %d/%d pods ready", readyReplicas, replicas))
			}
		}
	}

	// Emit health events for Target Allocator if enabled
	if changed.Spec.TargetAllocator.Enabled {
		taObjKey := client.ObjectKey{
			Namespace: changed.GetNamespace(),
			Name:      naming.TargetAllocator(changed.Name),
		}

		taObj := &appsv1.Deployment{}
		if err := cli.Get(ctx, taObjKey, taObj); err == nil {
			taReplicas := taObj.Status.Replicas
			taReadyReplicas := taObj.Status.ReadyReplicas

			// Emit Target Allocator events (simplified logic like other components)
			if taReplicas > 0 {
				if taReadyReplicas == taReplicas {
					// All Target Allocator pods are ready - emit Normal event
					recorder.Event(changed, "Normal", "ComponentHealthy",
						fmt.Sprintf("Target Allocator is healthy: %d/%d pods ready", taReadyReplicas, taReplicas))
				} else if taReadyReplicas == 0 {
					// No Target Allocator pods are ready - emit Warning event
					recorder.Event(changed, "Warning", "ComponentUnhealthy",
						fmt.Sprintf("Target Allocator is unhealthy: %d/%d pods ready", taReadyReplicas, taReplicas))
				} else {
					// Some Target Allocator pods are ready - emit Warning event
					recorder.Event(changed, "Warning", "ComponentPartiallyHealthy",
						fmt.Sprintf("Target Allocator is partially healthy: %d/%d pods ready", taReadyReplicas, taReplicas))
				}
			}
		}
	}

	return nil
}
