// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestBuild(t *testing.T) {
	logger := logr.Discard()
	tests := []struct {
		name            string
		params          manifests.Params
		expectedObjects int
		wantErr         bool
	}{
		{
			name: "deployment mode builds expected manifests",
			params: manifests.Params{
				Log: logger,
				OtelCol: v1alpha1.AmazonCloudWatchAgent{
					Spec: v1alpha1.AmazonCloudWatchAgentSpec{
						Mode:   v1alpha1.ModeDeployment,
						Config: "{\"agent\":\"\"}",
					},
				},
				Config: config.New(),
			},
			expectedObjects: 4, // ConfigMap, ServiceAccount, Deployment, MonitoringService
			wantErr:         false,
		},
		{
			name: "statefulset mode builds expected manifests",
			params: manifests.Params{
				Log: logger,
				OtelCol: v1alpha1.AmazonCloudWatchAgent{
					Spec: v1alpha1.AmazonCloudWatchAgentSpec{
						Mode:   v1alpha1.ModeStatefulSet,
						Config: "{\"agent\":\"\"}",
					},
				},
				Config: config.New(),
			},
			expectedObjects: 4, // ConfigMap, ServiceAccount, StatefulSet, MonitoringService
			wantErr:         false,
		},
		{
			name: "sidecar mode skips deployment manifests",
			params: manifests.Params{
				Log: logger,
				OtelCol: v1alpha1.AmazonCloudWatchAgent{
					Spec: v1alpha1.AmazonCloudWatchAgentSpec{
						Mode:   v1alpha1.ModeSidecar,
						Config: "{\"agent\":\"\"}",
					},
				},
				Config: config.New(),
			},
			expectedObjects: 3, // ConfigMap, ServiceAccount, MonitoringService
			wantErr:         false,
		},
		{
			name: "disabled services are not created",
			params: manifests.Params{
				Log: logger,
				OtelCol: v1alpha1.AmazonCloudWatchAgent{
					Spec: v1alpha1.AmazonCloudWatchAgentSpec{
						Mode:              v1alpha1.ModeDeployment,
						MonitoringService: v1alpha1.ServiceSpec{Enabled: ptr.To(false)},
						Config:            "{\"agent\":\"\"}",
					},
				},
				Config: config.New(),
			},
			expectedObjects: 3, // ConfigMap, ServiceAccount, Deployment
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			objects, err := Build(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, objects, tt.expectedObjects)
		})
	}
}
