// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
)

func TestPDB(t *testing.T) {
	type test struct {
		name           string
		MinAvailable   *intstr.IntOrString
		MaxUnavailable *intstr.IntOrString
	}
	tests := []test{
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

	otelcols := []v1beta1.AmazonCloudWatchAgent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
		},
	}

	for _, otelcol := range otelcols {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				otelcol.Spec.PodDisruptionBudget = &v1beta1.PodDisruptionBudgetSpec{
					MinAvailable:   test.MinAvailable,
					MaxUnavailable: test.MaxUnavailable,
				}
				configuration := config.New()
				pdb, err := PodDisruptionBudget(manifests.Params{
					Log:     logger,
					Config:  configuration,
					OtelCol: otelcol,
				})
				require.NoError(t, err)

				// verify
				assert.Equal(t, "my-instance", pdb.Name)
				assert.Equal(t, "my-instance", pdb.Labels["app.kubernetes.io/name"])
				assert.Equal(t, test.MinAvailable, pdb.Spec.MinAvailable)
				assert.Equal(t, test.MaxUnavailable, pdb.Spec.MaxUnavailable)
			})
		}
	}
}
