// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	testScheme = scheme.Scheme
)

func TestCollectorDefaultingWebhook(t *testing.T) {
	one := int32(1)
	five := int32(5)
	defaultCPUTarget := int32(90)

	if err := AddToScheme(testScheme); err != nil {
		fmt.Printf("failed to register scheme: %v", err)
		os.Exit(1)
	}

	tests := []struct {
		name     string
		otelcol  AmazonCloudWatchAgent
		expected AmazonCloudWatchAgent
	}{
		{
			name:    "all fields default",
			otelcol: AmazonCloudWatchAgent{},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						ManagementState: ManagementStateManaged,
						Replicas:        &one,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
					Mode:            ModeDeployment,
					UpgradeStrategy: UpgradeStrategyAutomatic,
				},
			},
		},
		{
			name: "provided values in spec",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeSidecar,
					UpgradeStrategy: "adhoc",
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas: &five,
					},
				},
			},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeSidecar,
					UpgradeStrategy: "adhoc",
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas:        &five,
						ManagementState: ManagementStateManaged,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
				},
			},
		},
		{
			name: "doesn't override unmanaged",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeSidecar,
					UpgradeStrategy: "adhoc",
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas:        &five,
						ManagementState: ManagementStateUnmanaged,
					},
				},
			},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeSidecar,
					UpgradeStrategy: "adhoc",
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas:        &five,
						ManagementState: ManagementStateUnmanaged,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
				},
			},
		},
		{
			name: "Setting Autoscaler MaxReplicas",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &five,
						MinReplicas: &one,
					},
				},
			},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeDeployment,
					UpgradeStrategy: UpgradeStrategyAutomatic,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas:        &one,
						ManagementState: ManagementStateManaged,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
					Autoscaler: &AutoscalerSpec{
						TargetCPUUtilization: &defaultCPUTarget,
						MaxReplicas:          &five,
						MinReplicas:          &one,
					},
				},
			},
		},
		{
			name: "Missing route termination",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeDeployment,
					Ingress: Ingress{
						Type: IngressTypeRoute,
					},
				},
			},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeDeployment,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						ManagementState: ManagementStateManaged,
						Replicas:        &one,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1,
							},
						},
					},
					Ingress: Ingress{
						Type: IngressTypeRoute,
						Route: OpenShiftRoute{
							Termination: TLSRouteTerminationTypeEdge,
						},
					},
					UpgradeStrategy: UpgradeStrategyAutomatic,
				},
			},
		},
		{
			name: "Defined PDB for collector",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeDeployment,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MinAvailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "10%",
							},
						},
					},
				},
			},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeDeployment,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas:        &one,
						ManagementState: ManagementStateManaged,
						PodDisruptionBudget: &PodDisruptionBudgetSpec{
							MinAvailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "10%",
							},
						},
					},
					UpgradeStrategy: UpgradeStrategyAutomatic,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			cvw := &CollectorWebhook{
				logger: logr.Discard(),
				scheme: testScheme,
			}
			ctx := context.Background()
			err := cvw.Default(ctx, &test.otelcol)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, test.otelcol)
		})
	}
}

var cfgYaml = `receivers:
 examplereceiver:
   endpoint: "0.0.0.0:12345"
 examplereceiver/settings:
   endpoint: "0.0.0.0:12346"
 prometheus:
   config:
     scrape_configs:
       - job_name: otel-collector
         scrape_interval: 10s
 jaeger/custom:
   protocols:
     thrift_http:
       endpoint: 0.0.0.0:15268
`

func TestOTELColValidatingWebhook(t *testing.T) {
	minusOne := int32(-1)
	zero := int32(0)
	zero64 := int64(0)
	one := int32(1)
	three := int32(3)
	five := int32(5)

	cfg := ""

	tests := []struct { //nolint:govet
		name             string
		otelcol          AmazonCloudWatchAgent
		expectedErr      string
		expectedWarnings []string
		shouldFailSar    bool
	}{
		{
			name:    "valid empty spec",
			otelcol: AmazonCloudWatchAgent{},
		},
		{
			name: "valid full spec",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeStatefulSet,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas: &three,
						Ports: []PortsSpec{
							{
								ServicePort: v1.ServicePort{
									Name: "port1",
									Port: 5555,
								},
							},
							{
								ServicePort: v1.ServicePort{
									Name:     "port2",
									Port:     5554,
									Protocol: v1.ProtocolUDP,
								},
							},
						},
					},
					Autoscaler: &AutoscalerSpec{
						MinReplicas: &one,
						MaxReplicas: &five,
						Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
							ScaleDown: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: &three,
							},
							ScaleUp: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: &five,
							},
						},
						TargetCPUUtilization: &five,
					},
					UpgradeStrategy: "adhoc",
					Config:          cfg,
				},
			},
		},
		{
			name: "invalid mode with volume claim templates",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					StatefulSetCommonFields: StatefulSetCommonFields{
						VolumeClaimTemplates: []v1.PersistentVolumeClaim{{}, {}},
					},
				},
			},
			expectedErr: "does not support the attribute 'volumeClaimTemplates'",
		},
		{
			name: "invalid mode with tolerations",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Tolerations: []v1.Toleration{{}, {}},
					},
				},
			},
			expectedErr: "does not support the attribute 'tolerations'",
		},
		{
			name: "invalid port name",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Ports: []PortsSpec{
							{
								ServicePort: v1.ServicePort{
									// this port name contains a non alphanumeric character, which is invalid.
									Name:     "-test🦄port",
									Port:     12345,
									Protocol: v1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec Ports configuration is incorrect",
		},
		{
			name: "invalid port name, too long",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Ports: []PortsSpec{
							{
								ServicePort: v1.ServicePort{
									Name: "aaaabbbbccccdddd", // len: 16, too long
									Port: 5555,
								},
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec Ports configuration is incorrect",
		},
		{
			name: "invalid port num",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Ports: []PortsSpec{
							{
								ServicePort: v1.ServicePort{
									Name: "aaaabbbbccccddd", // len: 15
									// no port set means it's 0, which is invalid
								},
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec Ports configuration is incorrect",
		},
		{
			name: "invalid max replicas",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &zero,
					},
				},
			},
			expectedErr: "maxReplicas should be defined and one or more",
		},
		{
			name: "invalid replicas, greater than max",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						Replicas: &five,
					},
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
					},
				},
			},
			expectedErr: "replicas must not be greater than maxReplicas",
		},
		{
			name: "invalid min replicas, greater than max",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						MinReplicas: &five,
					},
				},
			},
			expectedErr: "minReplicas must not be greater than maxReplicas",
		},
		{
			name: "invalid min replicas, lesser than 1",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						MinReplicas: &zero,
					},
				},
			},
			expectedErr: "minReplicas should be one or more",
		},
		{
			name: "invalid autoscaler scale down",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
							ScaleDown: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: &zero,
							},
						},
					},
				},
			},
			expectedErr: "scaleDown should be one or more",
		},
		{
			name: "invalid autoscaler scale up",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
							ScaleUp: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: &zero,
							},
						},
					},
				},
			},
			expectedErr: "scaleUp should be one or more",
		},
		{
			name: "invalid autoscaler target cpu utilization",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas:          &three,
						TargetCPUUtilization: &zero,
					},
				},
			},
			expectedErr: "targetCPUUtilization should be greater than 0 and less than 100",
		},
		{
			name: "autoscaler minReplicas is less than maxReplicas",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &one,
						MinReplicas: &five,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec autoscale configuration is incorrect, minReplicas must not be greater than maxReplicas",
		},
		{
			name: "invalid autoscaler metric type",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						Metrics: []MetricSpec{
							{
								Type: autoscalingv2.ResourceMetricSourceType,
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec autoscale configuration is incorrect, metric type unsupported. Expected metric of source type Pod",
		},
		{
			name: "invalid pod metric average value",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						Metrics: []MetricSpec{
							{
								Type: autoscalingv2.PodsMetricSourceType,
								Pods: &autoscalingv2.PodsMetricSource{
									Metric: autoscalingv2.MetricIdentifier{
										Name: "custom1",
									},
									Target: autoscalingv2.MetricTarget{
										Type:         autoscalingv2.AverageValueMetricType,
										AverageValue: resource.NewQuantity(int64(0), resource.DecimalSI),
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec autoscale configuration is incorrect, average value should be greater than 0",
		},
		{
			name: "utilization target is not valid with pod metrics",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Autoscaler: &AutoscalerSpec{
						MaxReplicas: &three,
						Metrics: []MetricSpec{
							{
								Type: autoscalingv2.PodsMetricSourceType,
								Pods: &autoscalingv2.PodsMetricSource{
									Metric: autoscalingv2.MetricIdentifier{
										Name: "custom1",
									},
									Target: autoscalingv2.MetricTarget{
										Type:               autoscalingv2.UtilizationMetricType,
										AverageUtilization: &one,
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec autoscale configuration is incorrect, invalid pods target type",
		},
		{
			name: "invalid deployment mode incompabible with ingress settings",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					Ingress: Ingress{
						Type: IngressTypeNginx,
					},
				},
			},
			expectedErr: fmt.Sprintf("Ingress can only be used in combination with the modes: %s, %s, %s",
				ModeDeployment, ModeDaemonSet, ModeStatefulSet,
			),
		},
		{
			name: "invalid mode with priorityClassName",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						PriorityClassName: "test-class",
					},
				},
			},
			expectedErr: "does not support the attribute 'priorityClassName'",
		},
		{
			name: "invalid mode with affinity",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
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
					},
				},
			},
			expectedErr: "does not support the attribute 'affinity'",
		},
		{
			name: "invalid InitialDelaySeconds",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						InitialDelaySeconds: &minusOne,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe InitialDelaySeconds configuration is incorrect",
		},
		{
			name: "invalid InitialDelaySeconds readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						InitialDelaySeconds: &minusOne,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe InitialDelaySeconds configuration is incorrect",
		},
		{
			name: "invalid PeriodSeconds",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						PeriodSeconds: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe PeriodSeconds configuration is incorrect",
		},
		{
			name: "invalid PeriodSeconds readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						PeriodSeconds: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe PeriodSeconds configuration is incorrect",
		},
		{
			name: "invalid TimeoutSeconds",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						TimeoutSeconds: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe TimeoutSeconds configuration is incorrect",
		},
		{
			name: "invalid TimeoutSeconds readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						TimeoutSeconds: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe TimeoutSeconds configuration is incorrect",
		},
		{
			name: "invalid SuccessThreshold",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						SuccessThreshold: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe SuccessThreshold configuration is incorrect",
		},
		{
			name: "invalid SuccessThreshold readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						SuccessThreshold: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe SuccessThreshold configuration is incorrect",
		},
		{
			name: "invalid FailureThreshold",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						FailureThreshold: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe FailureThreshold configuration is incorrect",
		},
		{
			name: "invalid FailureThreshold readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						FailureThreshold: &zero,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe FailureThreshold configuration is incorrect",
		},
		{
			name: "invalid TerminationGracePeriodSeconds",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					LivenessProbe: &Probe{
						TerminationGracePeriodSeconds: &zero64,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec LivenessProbe TerminationGracePeriodSeconds configuration is incorrect",
		},
		{
			name: "invalid TerminationGracePeriodSeconds readiness",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					ReadinessProbe: &Probe{
						TerminationGracePeriodSeconds: &zero64,
					},
				},
			},
			expectedErr: "the OpenTelemetry Spec ReadinessProbe TerminationGracePeriodSeconds configuration is incorrect",
		},
		{
			name: "invalid AdditionalContainers",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeSidecar,
					AmazonCloudWatchAgentCommonFields: AmazonCloudWatchAgentCommonFields{
						AdditionalContainers: []v1.Container{
							{
								Name: "test",
							},
						},
					},
				},
			},
			expectedErr: "the AmazonCloudWatchAgent mode is set to sidecar, which does not support the attribute 'AdditionalContainers'",
		},
		{
			name: "missing ingress hostname for subdomain ruleType",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Ingress: Ingress{
						RuleType: IngressRuleTypeSubdomain,
					},
				},
			},
			expectedErr: "a valid Ingress hostname has to be defined for subdomain ruleType",
		},
		{
			name: "invalid updateStrategy for Deployment mode",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeDeployment,
					DaemonSetUpdateStrategy: appsv1.DaemonSetUpdateStrategy{
						Type: "RollingUpdate",
						RollingUpdate: &appsv1.RollingUpdateDaemonSet{
							MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
							MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Collector mode is set to deployment, which does not support the attribute 'updateStrategy'",
		},
		{
			name: "invalid updateStrategy for Statefulset mode",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode: ModeStatefulSet,
					DeploymentUpdateStrategy: appsv1.DeploymentStrategy{
						Type: "RollingUpdate",
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
							MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
						},
					},
				},
			},
			expectedErr: "the OpenTelemetry Collector mode is set to statefulset, which does not support the attribute 'deploymentUpdateStrategy'",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			cvw := &CollectorWebhook{
				logger: logr.Discard(),
				scheme: testScheme,
			}
			ctx := context.Background()
			warnings, err := cvw.ValidateCreate(ctx, &test.otelcol)
			if test.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				fmt.Println(err)
				assert.ErrorContains(t, err, test.expectedErr)
			}
			assert.Equal(t, len(test.expectedWarnings), len(warnings))
			assert.ElementsMatch(t, warnings, test.expectedWarnings)
		})
	}
}
