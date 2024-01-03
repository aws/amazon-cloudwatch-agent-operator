// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	policyV1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func PodDisruptionBudget(params manifests.Params) *policyV1.PodDisruptionBudget {
	// defaulting webhook should always set this, but if unset then return nil.
	if params.OtelCol.Spec.PodDisruptionBudget == nil {
		params.Log.Info("pdb field is unset in Spec, skipping podDisruptionBudget creation")
		return nil
	}

	name := naming.Collector(params.OtelCol.Name)
	labels := manifestutils.Labels(params.OtelCol.ObjectMeta, name, params.OtelCol.Spec.Image, ComponentAmazonCloudWatchAgent, params.Config.LabelsFilter())
	annotations := Annotations(params.OtelCol)

	objectMeta := metav1.ObjectMeta{
		Name:        naming.PodDisruptionBudget(params.OtelCol.Name),
		Namespace:   params.OtelCol.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}

	return &policyV1.PodDisruptionBudget{
		ObjectMeta: objectMeta,
		Spec: policyV1.PodDisruptionBudgetSpec{
			MinAvailable:   params.OtelCol.Spec.PodDisruptionBudget.MinAvailable,
			MaxUnavailable: params.OtelCol.Spec.PodDisruptionBudget.MaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: objectMeta.Labels,
			},
		},
	}
}
