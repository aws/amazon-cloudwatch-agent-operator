// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

// TestTargetAllocatorMonitorSelectorValidation checks that the webhook rejects
// invalid ServiceMonitor/PodMonitor label selectors, which the CRD schema
// accepts, and accepts valid ones.
func TestTargetAllocatorMonitorSelectorValidation(t *testing.T) {
	req := func(key string, op metav1.LabelSelectorOperator, values ...string) *metav1.LabelSelector {
		return &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: key, Operator: op, Values: values}}}
	}
	valid := &metav1.LabelSelector{
		MatchLabels: map[string]string{"release": "prod"},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"payments"}},
			{Key: "tier", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"db"}},
			{Key: "app", Operator: metav1.LabelSelectorOpExists},
			{Key: "legacy", Operator: metav1.LabelSelectorOpDoesNotExist},
		},
	}
	invalidCases := []struct {
		name     string
		selector *metav1.LabelSelector
		wantErr  string
	}{
		{"unknown operator", req("team", "Equals", "payments"), `"Equals" is not a valid label selector operator`},
		{"In without values", req("team", metav1.LabelSelectorOpIn), "values set can't be empty"},
		{"NotIn without values", req("team", metav1.LabelSelectorOpNotIn), "values set can't be empty"},
		{"Exists with values", req("team", metav1.LabelSelectorOpExists, "payments"), "values set must be empty"},
		{"invalid label key", &metav1.LabelSelector{MatchLabels: map[string]string{"not a key": "x"}}, "name part must consist of"},
		{"invalid label value", &metav1.LabelSelector{MatchLabels: map[string]string{"team": "not a value"}}, "a valid label must be"},
	}

	type tc struct {
		name      string
		sm, pm    *metav1.LabelSelector
		wantField string
		wantErr   string
	}
	cases := []tc{
		{name: "no selectors"},
		{name: "empty selectors", sm: &metav1.LabelSelector{}, pm: &metav1.LabelSelector{}},
		{name: "valid selectors", sm: valid, pm: valid},
	}
	for _, ic := range invalidCases {
		cases = append(cases,
			tc{name: "serviceMonitorSelector " + ic.name, sm: ic.selector, wantField: "invalid spec.targetAllocator.prometheusCR.serviceMonitorSelector: ", wantErr: ic.wantErr},
			tc{name: "podMonitorSelector " + ic.name, pm: ic.selector, wantField: "invalid spec.targetAllocator.prometheusCR.podMonitorSelector: ", wantErr: ic.wantErr},
		)
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AmazonCloudWatchAgent{Spec: AmazonCloudWatchAgentSpec{
				Mode: ModeStatefulSet,
				TargetAllocator: AmazonCloudWatchAgentTargetAllocator{
					Enabled: true,
					PrometheusCR: AmazonCloudWatchAgentTargetAllocatorPrometheusCR{
						Enabled:                true,
						ServiceMonitorSelector: tt.sm,
						PodMonitorSelector:     tt.pm,
					},
				},
			}}
			cvw := &CollectorWebhook{
				logger: logr.Discard(),
				scheme: testScheme,
				cfg:    config.New(config.WithCollectorImage("collector:v0.0.0"), config.WithTargetAllocatorImage("ta:v0.0.0")),
			}
			for _, validate := range []func() error{
				func() error { _, err := cvw.ValidateCreate(context.Background(), agent); return err },
				func() error { _, err := cvw.ValidateUpdate(context.Background(), agent, agent); return err },
			} {
				err := validate()
				if tt.wantErr == "" {
					require.NoError(t, err)
					continue
				}
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantField)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
