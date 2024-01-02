// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"

	policyV1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodDisruptionBudget(params manifests.Params) (*policyV1.PodDisruptionBudget, error) {
	// defaulting webhook should set this if the strategy is compatible, but if unset then return nil.
	if params.OtelCol.Spec.TargetAllocator.PodDisruptionBudget == nil {
		params.Log.Info("pdb field is unset in Spec, skipping podDisruptionBudget creation")
		return nil, nil
	}

	// defaulter doesn't set PodDisruptionBudget if the strategy isn't valid,
	// if PodDisruptionBudget != nil and stategy isn't correct, users have set
	// it wrongly
	if params.OtelCol.Spec.TargetAllocator.AllocationStrategy != v1alpha1.OpenTelemetryTargetAllocatorAllocationStrategyConsistentHashing {
		params.Log.V(4).Info("current allocation strategy not compatible, skipping podDisruptionBudget creation")
		return nil, fmt.Errorf("target allocator pdb has been configured but the allocation strategy isn't not compatible")
	}

	name := naming.TAPodDisruptionBudget(params.OtelCol.Name)
	labels := Labels(params.OtelCol, name)

	annotations := Annotations(params.OtelCol, nil)

	objectMeta := metav1.ObjectMeta{
		Name:        name,
		Namespace:   params.OtelCol.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}

	return &policyV1.PodDisruptionBudget{
		ObjectMeta: objectMeta,
		Spec: policyV1.PodDisruptionBudgetSpec{
			MinAvailable:   params.OtelCol.Spec.TargetAllocator.PodDisruptionBudget.MinAvailable,
			MaxUnavailable: params.OtelCol.Spec.TargetAllocator.PodDisruptionBudget.MaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: SelectorLabels(params.OtelCol),
			},
		},
	}, nil
}
