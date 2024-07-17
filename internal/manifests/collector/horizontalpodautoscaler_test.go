// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
)

var logger = logf.Log.WithName("unit-tests")

func TestHPA(t *testing.T) {
	type test struct {
		name string
	}
	v2Test := test{}
	tests := []test{v2Test}

	var minReplicas int32 = 3
	var maxReplicas int32 = 5
	var cpuUtilization int32 = 66
	var memoryUtilization int32 = 77

	otelcols := []v1beta1.AmazonCloudWatchAgent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
			Spec: v1beta1.AmazonCloudWatchAgentSpec{
				Autoscaler: &v1beta1.AutoscalerSpec{
					MinReplicas:             &minReplicas,
					MaxReplicas:             &maxReplicas,
					TargetCPUUtilization:    &cpuUtilization,
					TargetMemoryUtilization: &memoryUtilization,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
			Spec: v1beta1.AmazonCloudWatchAgentSpec{
				Autoscaler: &v1beta1.AutoscalerSpec{
					MinReplicas:             &minReplicas,
					MaxReplicas:             &maxReplicas,
					TargetCPUUtilization:    &cpuUtilization,
					TargetMemoryUtilization: &memoryUtilization,
				},
			},
		},
	}

	for _, otelcol := range otelcols {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				configuration := config.New()
				params := manifests.Params{
					Config: configuration,
					OtelCol: v1beta1.AmazonCloudWatchAgent{
						ObjectMeta: otelcol.ObjectMeta,
						Spec: v1beta1.AmazonCloudWatchAgentSpec{
							Autoscaler: &v1beta1.AutoscalerSpec{
								MinReplicas:             otelcol.Spec.Autoscaler.MinReplicas,
								MaxReplicas:             otelcol.Spec.Autoscaler.MaxReplicas,
								TargetCPUUtilization:    otelcol.Spec.Autoscaler.TargetCPUUtilization,
								TargetMemoryUtilization: otelcol.Spec.Autoscaler.TargetMemoryUtilization,
							},
						},
					},
					Log: logger,
				}
				hpa, err := HorizontalPodAutoscaler(params)
				require.NoError(t, err)

				// verify
				assert.Equal(t, "my-instance", hpa.Name)
				assert.Equal(t, "my-instance", hpa.Labels["app.kubernetes.io/name"])
				assert.Equal(t, int32(3), *hpa.Spec.MinReplicas)
				assert.Equal(t, int32(5), hpa.Spec.MaxReplicas)
				assert.Equal(t, 2, len(hpa.Spec.Metrics))

				for _, metric := range hpa.Spec.Metrics {
					if metric.Resource.Name == corev1.ResourceCPU {
						assert.Equal(t, cpuUtilization, *metric.Resource.Target.AverageUtilization)
					} else if metric.Resource.Name == corev1.ResourceMemory {
						assert.Equal(t, memoryUtilization, *metric.Resource.Target.AverageUtilization)
					}
				}
			})
		}
	}

}
