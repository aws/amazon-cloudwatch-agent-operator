// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

const taWarning = "we do not recommend enabling Target Allocator when not running as a StatefulSet"

// TestTargetAllocatorModeValidation covers which mode/strategy combinations the
// webhook rejects, and when it warns about running the Target Allocator outside
// a StatefulSet.
func TestTargetAllocatorModeValidation(t *testing.T) {
	perNode := AmazonCloudWatchAgentTargetAllocatorAllocationStrategyPerNode
	hashing := AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing

	tests := []struct {
		name        string
		mode        Mode
		strategy    AmazonCloudWatchAgentTargetAllocatorAllocationStrategy
		scraperRole string
		wantErr     string
		wantWarning bool
	}{
		{name: "per-node on daemonset is accepted without warning", mode: ModeDaemonSet, strategy: perNode},
		{name: "per-node on deployment is rejected", mode: ModeDeployment, strategy: perNode, wantErr: "only supported in mode daemonset"},
		{name: "per-node on statefulset is rejected", mode: ModeStatefulSet, strategy: perNode, wantErr: "only supported in mode daemonset"},
		{name: "scraper role on deployment is accepted without warning", mode: ModeDeployment, strategy: hashing, scraperRole: "cluster-scraper"},
		{name: "consistent-hashing on deployment still warns", mode: ModeDeployment, strategy: hashing, wantWarning: true},
		{name: "consistent-hashing on daemonset still warns", mode: ModeDaemonSet, strategy: hashing, wantWarning: true},
		{name: "consistent-hashing on statefulset does not warn", mode: ModeStatefulSet, strategy: hashing},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			agent := &AmazonCloudWatchAgent{Spec: AmazonCloudWatchAgentSpec{
				Mode: tt.mode,
				TargetAllocator: AmazonCloudWatchAgentTargetAllocator{
					Enabled:            true,
					AllocationStrategy: tt.strategy,
					PrometheusCR:       AmazonCloudWatchAgentTargetAllocatorPrometheusCR{Enabled: true, ScraperRole: tt.scraperRole},
				},
			}}
			cvw := &CollectorWebhook{
				logger: logr.Discard(),
				scheme: testScheme,
				cfg:    config.New(config.WithCollectorImage("collector:v0.0.0"), config.WithTargetAllocatorImage("ta:v0.0.0")),
			}
			warnings, err := cvw.ValidateCreate(context.Background(), agent)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			warned := false
			for _, w := range warnings {
				if strings.Contains(w, taWarning) {
					warned = true
				}
			}
			assert.Equalf(t, tt.wantWarning, warned, "warnings: %v", warnings)
		})
	}
}

