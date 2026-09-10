// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	policyV1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// PodDisruptionBudget builds a PDB for the NeuronMonitor workload when
// Spec.PodDisruptionBudget is set. Returns nil otherwise.
func PodDisruptionBudget(params manifests.Params) *policyV1.PodDisruptionBudget {
	if params.NeuronExp.Spec.PodDisruptionBudget == nil {
		params.Log.Info("pdb field is unset in Spec, skipping podDisruptionBudget creation")
		return nil
	}

	name := params.NeuronExp.Name
	if len(name) == 0 {
		name = ComponentNeuronExporter
	}
	labels := manifestutils.Labels(params.NeuronExp.ObjectMeta, name, params.NeuronExp.Spec.Image, ComponentNeuronExporter, params.Config.LabelsFilter())
	annotations := Annotations(params.NeuronExp)

	return &policyV1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.PodDisruptionBudget(name),
			Namespace:   params.NeuronExp.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: policyV1.PodDisruptionBudgetSpec{
			MinAvailable:   params.NeuronExp.Spec.PodDisruptionBudget.MinAvailable,
			MaxUnavailable: params.NeuronExp.Spec.PodDisruptionBudget.MaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: manifestutils.SelectorLabels(params.NeuronExp.ObjectMeta, ComponentNeuronExporter),
			},
		},
	}
}
