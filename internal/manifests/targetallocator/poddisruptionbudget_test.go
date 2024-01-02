// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

type test struct {
	name           string
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
}

var tests = []test{
	{
		name: "MinAvailable-int",
		MinAvailable: &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: 1,
		},
	},
	{
		name: "MinAvailable-string",
		MinAvailable: &intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "10%",
		},
	},
	{
		name: "MaxUnavailable-int",
		MaxUnavailable: &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: 1,
		},
	},
	{
		name: "MaxUnavailable-string",
		MaxUnavailable: &intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "10%",
		},
	},
}

func TestPDBWithValidStrategy(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			otelcol := v1alpha1.AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-instance",
				},
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					TargetAllocator: v1alpha1.OpenTelemetryTargetAllocator{
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
							MinAvailable:   test.MinAvailable,
							MaxUnavailable: test.MaxUnavailable,
						},
						AllocationStrategy: v1alpha1.OpenTelemetryTargetAllocatorAllocationStrategyConsistentHashing,
					},
				},
			}
			configuration := config.New()
			pdb, err := PodDisruptionBudget(manifests.Params{
				Log:     logger,
				Config:  configuration,
				OtelCol: otelcol,
			})

			// verify
			assert.NoError(t, err)
			assert.Equal(t, "my-instance-targetallocator", pdb.Name)
			assert.Equal(t, "my-instance-targetallocator", pdb.Labels["app.kubernetes.io/name"])
			assert.Equal(t, test.MinAvailable, pdb.Spec.MinAvailable)
			assert.Equal(t, test.MaxUnavailable, pdb.Spec.MaxUnavailable)
		})
	}
}

func TestPDBWithNotValidStrategy(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			otelcol := v1alpha1.AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-instance",
				},
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					TargetAllocator: v1alpha1.OpenTelemetryTargetAllocator{
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
							MinAvailable:   test.MinAvailable,
							MaxUnavailable: test.MaxUnavailable,
						},
						AllocationStrategy: v1alpha1.OpenTelemetryTargetAllocatorAllocationStrategyLeastWeighted,
					},
				},
			}
			configuration := config.New()
			pdb, err := PodDisruptionBudget(manifests.Params{
				Log:     logger,
				Config:  configuration,
				OtelCol: otelcol,
			})

			// verify
			assert.Error(t, err)
			assert.Nil(t, pdb)
		})
	}
}

func TestNoPDB(t *testing.T) {
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.OpenTelemetryTargetAllocator{
				AllocationStrategy: v1alpha1.OpenTelemetryTargetAllocatorAllocationStrategyLeastWeighted,
			},
		},
	}
	configuration := config.New()
	pdb, err := PodDisruptionBudget(manifests.Params{
		Log:     logger,
		Config:  configuration,
		OtelCol: otelcol,
	})

	// verify
	assert.NoError(t, err)
	assert.Nil(t, pdb)
}
