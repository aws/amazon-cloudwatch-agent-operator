// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

func UpdateCollectorStatus(ctx context.Context, cli client.Client, changed *v1alpha1.OpenTelemetryCollector) error {
	if changed.Status.Version == "" {
		// a version is not set, otherwise let the upgrade mechanism take care of it!
		changed.Status.Version = version.OpenTelemetryCollector()
	}
	mode := changed.Spec.Mode
	if mode != v1alpha1.ModeDeployment && mode != v1alpha1.ModeStatefulSet {
		changed.Status.Scale.Replicas = 0
		changed.Status.Scale.Selector = ""
		return nil
	}

	name := naming.Collector(changed.Name)

	// Set the scale selector
	labels := manifestutils.Labels(changed.ObjectMeta, name, changed.Spec.Image, collector.ComponentOpenTelemetryCollector, []string{})
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
		statusImage = obj.Spec.Template.Spec.Containers[0].Image
	}
	changed.Status.Scale.Replicas = replicas
	changed.Status.Image = statusImage
	changed.Status.Scale.StatusReplicas = statusReplicas

	return nil
}
