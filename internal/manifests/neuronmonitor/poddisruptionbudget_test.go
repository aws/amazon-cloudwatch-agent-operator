// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestNeuronPodDisruptionBudgetNilSpecSkips(t *testing.T) {
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.Nil(t, pdb)
}

func TestNeuronPodDisruptionBudgetMinAvailable(t *testing.T) {
	minAvailable := intstr.FromInt(1)
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
			},
		},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.NotNil(t, pdb)
	assert.Equal(t, "neuron", pdb.Name)
	assert.Equal(t, &minAvailable, pdb.Spec.MinAvailable)
	assert.Equal(t, ComponentNeuronExporter, pdb.Spec.Selector.MatchLabels["app.kubernetes.io/component"])
}

func TestNeuronPodDisruptionBudgetMaxUnavailablePercent(t *testing.T) {
	maxUnavailable := intstr.FromString("50%")
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
			},
		},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.NotNil(t, pdb)
	assert.Equal(t, &maxUnavailable, pdb.Spec.MaxUnavailable)
}
