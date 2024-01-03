// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func upgrade0_56_0(u VersionUpgrade, otelcol *v1alpha1.AmazonCloudWatchAgent) (*v1alpha1.AmazonCloudWatchAgent, error) {
	// return if this does not use an autoscaler
	if otelcol.Spec.MaxReplicas == nil {
		return otelcol, nil
	}

	// Add minReplicas
	one := int32(1)
	otelcol.Spec.MinReplicas = &one

	// Find the existing HPA for this collector and upgrade it if necessary
	listOptions := []client.ListOption{
		client.InNamespace(otelcol.Namespace),
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", otelcol.Namespace, otelcol.Name),
			"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		}),
	}

	hpaList := &autoscalingv1.HorizontalPodAutoscalerList{}
	ctx := context.Background()
	if err := u.Client.List(ctx, hpaList, listOptions...); err != nil {
		return nil, fmt.Errorf("couldn't upgrade to v0.56.0, failed trying to find HPA instances: %w", err)
	}

	errors := []error{}
	for i := range hpaList.Items {
		existing := hpaList.Items[i]
		// If there is an autoscaler based on Deployment, replace it with one based on AmazonCloudWatchAgent
		if existing.Spec.ScaleTargetRef.Kind == "Deployment" {
			updated := existing.DeepCopy()
			updated.Spec.ScaleTargetRef = autoscalingv1.CrossVersionObjectReference{
				Kind:       "AmazonCloudWatchAgent",
				Name:       naming.AmazonCloudWatchAgentName(otelcol.Name),
				APIVersion: v1alpha1.GroupVersion.String(),
			}
			patch := client.MergeFrom(&existing)
			err := u.Client.Patch(ctx, updated, patch)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) != 0 {
		return nil, fmt.Errorf("couldn't upgrade to v0.56.0, failed to recreate autoscaler: %v", errors)
	}

	u.Log.Info("in upgrade0_56_0", "Otel Instance", otelcol.Name, "Upgrade version", u.Version.String())
	u.Recorder.Event(otelcol, "Normal", "Upgrade", "upgraded to v0.56.0, added minReplicas. recreated HPA instance")

	return otelcol, nil
}
