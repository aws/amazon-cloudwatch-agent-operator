// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestTargetAllocator(t *testing.T) {
	objectMetadata := metav1.ObjectMeta{
		Name:      "name",
		Namespace: "namespace",
		Annotations: map[string]string{
			"annotation_key": "annotation_value",
		},
		Labels: map[string]string{
			"label_key": "label_value",
		},
	}
	replicas := int32(2)
	runAsNonRoot := true
	privileged := true
	runAsUser := int64(1337)
	runasGroup := int64(1338)
	otelcolConfig := v1alpha1.Config{
		Receivers: v1alpha1.AnyConfig{
			Object: map[string]interface{}{
				"prometheus": map[string]any{
					"config": map[string]any{
						"scrape_configs": []any{},
					},
				},
			},
		},
	}

	testCases := []struct {
		name    string
		input   v1alpha1.AmazonCloudWatchAgent
		want    *v1alpha1.TargetAllocator
		wantErr error
	}{
		{
			name: "disabled",
			input: v1alpha1.AmazonCloudWatchAgent{
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Enabled: false,
					},
				},
			},
			want: nil,
		},
		{
			name: "metadata",
			input: v1alpha1.AmazonCloudWatchAgent{
				ObjectMeta: objectMetadata,
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					Config: otelcolConfig,
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Enabled: true,
					},
				},
			},
			want: &v1alpha1.TargetAllocator{
				ObjectMeta: objectMetadata,
				Spec: v1alpha1.TargetAllocatorSpec{
					ScrapeConfigs: []v1alpha1.AnyConfig{},
				},
			},
		},
		{
			name: "full",
			input: v1alpha1.AmazonCloudWatchAgent{
				ObjectMeta: objectMetadata,
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
						Replicas:     &replicas,
						NodeSelector: map[string]string{"key": "value"},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
						AllocationStrategy: v1alpha1.TargetAllocatorAllocationStrategyConsistentHashing,
						FilterStrategy:     "relabel-config",
						ServiceAccount:     "serviceAccountName",
						Image:              "custom_image",
						Enabled:            true,
						Affinity: &v1.Affinity{
							NodeAffinity: &v1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
									NodeSelectorTerms: []v1.NodeSelectorTerm{
										{
											MatchExpressions: []v1.NodeSelectorRequirement{
												{
													Key:      "node",
													Operator: v1.NodeSelectorOpIn,
													Values:   []string{"test-node"},
												},
											},
										},
									},
								},
							},
						},
						PrometheusCR: v1alpha1.TargetAllocatorPrometheusCR{
							Enabled:        true,
							ScrapeInterval: &metav1.Duration{Duration: time.Second},
							PodMonitorSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"podmonitorkey": "podmonitorvalue"},
							},
							ServiceMonitorSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"servicemonitorkey": "servicemonitorkey"},
							},
						},
						PodSecurityContext: &v1.PodSecurityContext{
							RunAsNonRoot: &runAsNonRoot,
							RunAsUser:    &runAsUser,
							RunAsGroup:   &runasGroup,
						},
						SecurityContext: &v1.SecurityContext{
							RunAsUser:  &runAsUser,
							Privileged: &privileged,
						},
						TopologySpreadConstraints: []v1.TopologySpreadConstraint{
							{
								MaxSkew:           1,
								TopologyKey:       "kubernetes.io/hostname",
								WhenUnsatisfiable: "DoNotSchedule",
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
								},
							},
						},
						Tolerations: []v1.Toleration{
							{
								Key:    "hii",
								Value:  "greeting",
								Effect: "NoSchedule",
							},
						},
						Env: []v1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &v1.EnvVarSource{
									FieldRef: &v1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
						},
						Observability: v1alpha1.ObservabilitySpec{
							Metrics: v1alpha1.MetricsConfigSpec{
								EnableMetrics: true,
							},
						},
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
					Config: otelcolConfig,
				},
			},
			want: &v1alpha1.TargetAllocator{
				ObjectMeta: objectMetadata,
				Spec: v1alpha1.TargetAllocatorSpec{
					AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
						Replicas:     &replicas,
						NodeSelector: map[string]string{"key": "value"},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
						ServiceAccount: "serviceAccountName",
						Image:          "custom_image",
						Affinity: &v1.Affinity{
							NodeAffinity: &v1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
									NodeSelectorTerms: []v1.NodeSelectorTerm{
										{
											MatchExpressions: []v1.NodeSelectorRequirement{
												{
													Key:      "node",
													Operator: v1.NodeSelectorOpIn,
													Values:   []string{"test-node"},
												},
											},
										},
									},
								},
							},
						},
						PodSecurityContext: &v1.PodSecurityContext{
							RunAsNonRoot: &runAsNonRoot,
							RunAsUser:    &runAsUser,
							RunAsGroup:   &runasGroup,
						},
						SecurityContext: &v1.SecurityContext{
							RunAsUser:  &runAsUser,
							Privileged: &privileged,
						},
						TopologySpreadConstraints: []v1.TopologySpreadConstraint{
							{
								MaxSkew:           1,
								TopologyKey:       "kubernetes.io/hostname",
								WhenUnsatisfiable: "DoNotSchedule",
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
								},
							},
						},
						Tolerations: []v1.Toleration{
							{
								Key:    "hii",
								Value:  "greeting",
								Effect: "NoSchedule",
							},
						},
						Env: []v1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &v1.EnvVarSource{
									FieldRef: &v1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
						},

						PodDisruptionBudget: &v1alpha1.PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
					AllocationStrategy: v1alpha1.TargetAllocatorAllocationStrategyConsistentHashing,
					FilterStrategy:     v1alpha1.TargetAllocatorFilterStrategyRelabelConfig,
					PrometheusCR: v1alpha1.TargetAllocatorPrometheusCR{
						Enabled:        true,
						ScrapeInterval: &metav1.Duration{Duration: time.Second},
						PodMonitorSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"podmonitorkey": "podmonitorvalue"},
						},
						ServiceMonitorSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"servicemonitorkey": "servicemonitorkey"},
						},
					},
					ScrapeConfigs: []v1alpha1.AnyConfig{},
					Observability: v1alpha1.ObservabilitySpec{
						Metrics: v1alpha1.MetricsConfigSpec{
							EnableMetrics: true,
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			params := manifests.Params{
				OtelCol: testCase.input,
			}
			actual, err := TargetAllocator(params)
			assert.Equal(t, testCase.wantErr, err)
			assert.Equal(t, testCase.want, actual)
		})
	}
}

func TestGetScrapeConfigs(t *testing.T) {
	testCases := []struct {
		name    string
		input   v1alpha1.Config
		want    []v1alpha1.AnyConfig
		wantErr error
	}{
		{
			name: "empty scrape configs list",
			input: v1alpha1.Config{
				Receivers: v1alpha1.AnyConfig{
					Object: map[string]interface{}{
						"prometheus": map[string]any{
							"config": map[string]any{
								"scrape_configs": []any{},
							},
						},
					},
				},
			},
			want: []v1alpha1.AnyConfig{},
		},
		{
			name: "no scrape configs key",
			input: v1alpha1.Config{
				Receivers: v1alpha1.AnyConfig{
					Object: map[string]interface{}{
						"prometheus": map[string]any{
							"config": map[string]any{},
						},
					},
				},
			},
			wantErr: fmt.Errorf("no scrape_configs available as part of the configuration"),
		},
		{
			name: "one scrape config",
			input: v1alpha1.Config{
				Receivers: v1alpha1.AnyConfig{
					Object: map[string]interface{}{
						"prometheus": map[string]any{
							"config": map[string]any{
								"scrape_configs": []any{
									map[string]any{
										"job": "somejob",
									},
								},
							},
						},
					},
				},
			},
			want: []v1alpha1.AnyConfig{
				{Object: map[string]interface{}{"job": "somejob"}},
			},
		},
		{
			name: "regex substitution",
			input: v1alpha1.Config{
				Receivers: v1alpha1.AnyConfig{
					Object: map[string]interface{}{
						"prometheus": map[string]any{
							"config": map[string]any{
								"scrape_configs": []any{
									map[string]any{
										"job": "somejob",
										"metric_relabel_configs": []map[string]any{
											{
												"action":      "labelmap",
												"regex":       "label_(.+)",
												"replacement": "$$1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []v1alpha1.AnyConfig{
				{Object: map[string]interface{}{
					"job": "somejob",
					"metric_relabel_configs": []any{
						map[any]any{
							"action":      "labelmap",
							"regex":       "label_(.+)",
							"replacement": "$1",
						},
					},
				}},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			configStr, err := testCase.input.Yaml()
			require.NoError(t, err)
			actual, err := getScrapeConfigs(configStr)
			assert.Equal(t, testCase.wantErr, err)
			assert.Equal(t, testCase.want, actual)
		})
	}
}
