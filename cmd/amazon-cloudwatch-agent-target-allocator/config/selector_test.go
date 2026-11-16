// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnmarshalMonitorSelectors(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		want    *metav1.LabelSelector
		wantErr string
	}{
		{
			name: "absent",
			yaml: "scraper_role: cluster-scraper\n",
			want: nil,
		},
		{
			name: "null",
			yaml: "service_monitor_selector:\n",
			want: nil,
		},
		{
			name: "empty map selects all",
			yaml: "service_monitor_selector: {}\n",
			want: &metav1.LabelSelector{},
		},
		{
			name: "legacy flat map is read as matchLabels",
			yaml: "service_monitor_selector:\n  release: test\n  team: a\n",
			want: &metav1.LabelSelector{MatchLabels: map[string]string{"release": "test", "team": "a"}},
		},
		{
			name: "camelCase matchLabels",
			yaml: "service_monitor_selector:\n  matchLabels:\n    release: test\n",
			want: &metav1.LabelSelector{MatchLabels: map[string]string{"release": "test"}},
		},
		{
			name: "lowercase matchlabels and matchexpressions",
			yaml: `service_monitor_selector:
  matchlabels:
    release: test
  matchexpressions:
  - key: env
    operator: NotIn
    values: [dev]
  - key: team
    operator: DoesNotExist
`,
			want: &metav1.LabelSelector{
				MatchLabels: map[string]string{"release": "test"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"dev"}},
					{Key: "team", Operator: metav1.LabelSelectorOpDoesNotExist},
				},
			},
		},
		{
			name: "camelCase wins over lowercase",
			yaml: `service_monitor_selector:
  matchlabels:
    release: lower
  matchLabels:
    release: camel
`,
			want: &metav1.LabelSelector{MatchLabels: map[string]string{"release": "camel"}},
		},
		{
			name:    "selector fields mixed with flat labels",
			yaml:    "service_monitor_selector:\n  matchLabels:\n    release: test\n  team: a\n",
			wantErr: "cannot mix label selector fields (matchLabels) with flat label keys (team)",
		},
		{
			name:    "not a map",
			yaml:    "service_monitor_selector: release=test\n",
			wantErr: "expected a map",
		},
		{
			name:    "unknown requirement field",
			yaml:    "service_monitor_selector:\n  matchExpressions:\n  - key: env\n    op: In\n",
			wantErr: "unknown field",
		},
		{
			name:    "legacy flat map with non-string value",
			yaml:    "service_monitor_selector:\n  release:\n    nested: value\n",
			wantErr: "service_monitor_selector",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CreateDefaultConfig()
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &got)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.ServiceMonitorSelector)
			assert.Nil(t, got.PodMonitorSelector)
		})
	}
}

// TestUnmarshalKeepsOtherFieldsStrict checks that decoding the selectors
// separately does not relax strict decoding of the rest of the config, and does
// not drop defaults for fields the file does not set.
func TestUnmarshalKeepsOtherFieldsStrict(t *testing.T) {
	got := CreateDefaultConfig()
	err := yaml.UnmarshalStrict([]byte("pod_monitor_selector:\n  release: test\nnot_a_field: true\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not_a_field")

	got = CreateDefaultConfig()
	require.NoError(t, yaml.UnmarshalStrict([]byte("pod_monitor_selector:\n  release: test\nscraper_role: cluster-scraper\n"), &got))
	assert.Equal(t, "cluster-scraper", got.ScraperRole)
	assert.Equal(t, CreateDefaultConfig().HTTPS, got.HTTPS)
	assert.Equal(t, CreateDefaultConfig().PrometheusCR, got.PrometheusCR)
	assert.Equal(t, &metav1.LabelSelector{MatchLabels: map[string]string{"release": "test"}}, got.PodMonitorSelector)
}

// TestSelectorRoundTripFromOperatorConfigMap marshals a selector the way the
// operator writes the Target Allocator ConfigMap (gopkg.in/yaml.v2 on a
// *metav1.LabelSelector, which yields lowercase field names) and checks that
// the Target Allocator decodes the same selector back.
func TestSelectorRoundTripFromOperatorConfigMap(t *testing.T) {
	want := &metav1.LabelSelector{
		MatchLabels: map[string]string{"release": "test"},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web", "api"}},
			{Key: "team", Operator: metav1.LabelSelectorOpExists},
		},
	}
	b, err := yaml.Marshal(map[string]interface{}{
		"service_monitor_selector": want,
		"pod_monitor_selector":     want,
	})
	require.NoError(t, err)

	got := CreateDefaultConfig()
	require.NoError(t, yaml.UnmarshalStrict(b, &got))
	// yaml.v2 writes a nil Values as an empty list, so compare the resulting
	// selectors rather than the structs.
	wantSel, err := MonitorSelector(want)
	require.NoError(t, err)
	for _, s := range []*metav1.LabelSelector{got.ServiceMonitorSelector, got.PodMonitorSelector} {
		gotSel, err := MonitorSelector(s)
		require.NoError(t, err)
		assert.Equal(t, wantSel.String(), gotSel.String())
	}
	assert.Equal(t, "release=test,team,tier in (api,web)", wantSel.String())
}
