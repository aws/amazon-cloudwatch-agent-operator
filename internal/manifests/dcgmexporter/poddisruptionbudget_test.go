// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestDcgmPodDisruptionBudgetNilSpecSkips(t *testing.T) {
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.Nil(t, pdb)
}

func TestDcgmPodDisruptionBudgetMinAvailable(t *testing.T) {
	minAvailable := intstr.FromInt(1)
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
			},
		},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.NotNil(t, pdb)
	assert.Equal(t, "dcgm", pdb.Name)
	assert.Equal(t, &minAvailable, pdb.Spec.MinAvailable)
	assert.Nil(t, pdb.Spec.MaxUnavailable)
	// selector must match dcgm-exporter component
	assert.Equal(t, ComponentDcgmExporter, pdb.Spec.Selector.MatchLabels["app.kubernetes.io/component"])
}

func TestDcgmPodDisruptionBudgetMaxUnavailablePercent(t *testing.T) {
	maxUnavailable := intstr.FromString("50%")
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
			},
		},
	}
	pdb := PodDisruptionBudget(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.NotNil(t, pdb)
	assert.Equal(t, &maxUnavailable, pdb.Spec.MaxUnavailable)
}
