// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"strconv"
	"time"

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
	var creationTime time.Time

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
		creationTime = obj.CreationTimestamp.Time

	case v1alpha1.ModeStatefulSet:
		obj := &appsv1.StatefulSet{}
		if err := cli.Get(ctx, objKey, obj); err != nil {
			return fmt.Errorf("failed to get statefulSet status.replicas: %w", err)
		}
		replicas = obj.Status.Replicas
		readyReplicas = obj.Status.ReadyReplicas
		statusReplicas = strconv.Itoa(int(readyReplicas)) + "/" + strconv.Itoa(int(replicas))
		statusImage = obj.Spec.Template.Spec.Containers[0].Image
		creationTime = obj.CreationTimestamp.Time

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
		creationTime = obj.CreationTimestamp.Time
	}
	changed.Status.Scale.Replicas = replicas
	changed.Status.Image = statusImage
	changed.Status.Scale.StatusReplicas = statusReplicas

	// Extract and set version from image tag (always prioritize image tag over default)
	if statusImage != "" {
		if imageVersion := manifestutils.ExtractVersionFromImage(statusImage); imageVersion != "" {
			changed.Status.Version = imageVersion
		}
	}

	// Always emit health events like DCGM and Neuron do
	manifestutils.EmitHealthEvents(recorder, changed, "CloudWatch Agent", readyReplicas, replicas, creationTime, 30*time.Second)

	// Always emit Target Allocator health events
	taObjKey := client.ObjectKey{
		Namespace: changed.GetNamespace(),
		Name:      naming.TargetAllocator(changed.Name),
	}

	taObj := &appsv1.Deployment{}
	var taReplicas, taReadyReplicas int32
	var taCreationTime time.Time
	if err := cli.Get(ctx, taObjKey, taObj); err == nil {
		taReplicas = taObj.Status.Replicas
		taReadyReplicas = taObj.Status.ReadyReplicas
		taCreationTime = taObj.CreationTimestamp.Time
	}
	// Always emit health events regardless of whether we can get the deployment
	manifestutils.EmitHealthEvents(recorder, changed, "Target Allocator", taReadyReplicas, taReplicas, taCreationTime, 30*time.Second)

	return nil
}
