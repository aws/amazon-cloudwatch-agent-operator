// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestAgentDefaultingWebhook(t *testing.T) {
	one := int32(1)
	five := int32(5)

	tests := []struct {
		name     string
		agent    AmazonCloudWatchAgent
		expected AmazonCloudWatchAgent
	}{
		{
			name:  "all fields default",
			agent: AmazonCloudWatchAgent{},
			expected: AmazonCloudWatchAgent{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					},
				},
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeDeployment,
					Replicas:        &one,
					UpgradeStrategy: UpgradeStrategyAutomatic,
				},
			},
		},
		{
			name: "provided values in spec",
			agent: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeSidecar,
					Replicas:        &five,
					UpgradeStrategy: "adhoc",
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
					Replicas:        &five,
					UpgradeStrategy: "adhoc",
				},
			},
		},
		{
			name: "Missing route termination",
			agent: AmazonCloudWatchAgent{
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
					Ingress: Ingress{
						Type: IngressTypeRoute,
						Route: OpenShiftRoute{
							Termination: TLSRouteTerminationTypeEdge,
						},
					},
					Replicas:        &one,
					UpgradeStrategy: UpgradeStrategyAutomatic,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.agent.Default()
			assert.Equal(t, test.expected, test.agent)
		})
	}
}

// TODO: a lot of these tests use .Spec.MaxReplicas and .Spec.MinReplicas. These fields are
// deprecated and moved to .Spec.Autoscaler. Fine to use these fields to test that old CRD is
// still supported but should eventually be updated.
func TestAgentValidatingWebhook(t *testing.T) {
	three := int32(3)

	tests := []struct { //nolint:govet
		name        string
		otelcol     AmazonCloudWatchAgent
		expectedErr string
	}{
		{
			name:    "valid empty spec",
			otelcol: AmazonCloudWatchAgent{},
		},
		{
			name: "valid full spec",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:            ModeStatefulSet,
					Replicas:        &three,
					UpgradeStrategy: "adhoc",
					Config: `"agent": {
   "metrics_collection_interval": 60,
   "region": "us-west-1",
   "logfile": "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log",
   "debug": false,
   "run_as_user": "cwagent"
}
`,
					Ports: []v1.ServicePort{
						{
							Name: "port1",
							Port: 5555,
						},
						{
							Name:     "port2",
							Port:     5554,
							Protocol: v1.ProtocolUDP,
						},
					},
				},
			},
		},
		{
			name: "invalid mode with volume claim templates",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:                 ModeSidecar,
					VolumeClaimTemplates: []v1.PersistentVolumeClaim{{}, {}},
				},
			},
			expectedErr: "does not support the attribute 'volumeClaimTemplates'",
		},
		{
			name: "invalid mode with tolerations",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Mode:        ModeSidecar,
					Tolerations: []v1.Toleration{{}, {}},
				},
			},
			expectedErr: "does not support the attribute 'tolerations'",
		},
		{
			name: "invalid port name",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Ports: []v1.ServicePort{
						{
							// this port name contains a non alphanumeric character, which is invalid.
							Name:     "-test🦄port",
							Port:     12345,
							Protocol: v1.ProtocolTCP,
						},
					},
				},
			},
			expectedErr: "the AmazonCloudWatchAgent Spec Ports configuration is incorrect",
		},
		{
			name: "invalid port name, too long",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Ports: []v1.ServicePort{
						{
							Name: "aaaabbbbccccdddd", // len: 16, too long
							Port: 5555,
						},
					},
				},
			},
			expectedErr: "the AmazonCloudWatchAgent Spec Ports configuration is incorrect",
		},
		{
			name: "invalid port num",
			otelcol: AmazonCloudWatchAgent{
				Spec: AmazonCloudWatchAgentSpec{
					Ports: []v1.ServicePort{
						{
							Name: "aaaabbbbccccddd", // len: 15
							// no port set means it's 0, which is invalid
						},
					},
				},
			},
			expectedErr: "the AmazonCloudWatchAgent Spec Ports configuration is incorrect",
		},
		{
			name: "invalid deployment mode incompatible with ingress settings",
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
					Mode:              ModeSidecar,
					PriorityClassName: "test-class",
				},
			},
			expectedErr: "does not support the attribute 'priorityClassName'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.otelcol.validateCRDSpec()
			if test.expectedErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, test.expectedErr)
		})
	}
}
