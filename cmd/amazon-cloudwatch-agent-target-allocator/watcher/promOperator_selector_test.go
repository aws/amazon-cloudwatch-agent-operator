// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

// TestGetSelector covers how a monitor label selector is turned into the
// labels.Selector used to list ServiceMonitors and PodMonitors.
func TestGetSelector(t *testing.T) {
	prodWeb := labels.Set{"env": "prod", "tier": "web"}
	devWeb := labels.Set{"env": "dev", "tier": "web"}
	prodDB := labels.Set{"env": "prod", "tier": "db"}
	noEnv := labels.Set{"tier": "web"}
	empty := labels.Set{}

	cases := []struct {
		name     string
		selector *metav1.LabelSelector
		matches  []labels.Set
		rejects  []labels.Set
	}{
		{
			name:     "nil selects all",
			selector: nil,
			matches:  []labels.Set{prodWeb, devWeb, prodDB, noEnv, empty},
		},
		{
			name:     "empty selects all",
			selector: &metav1.LabelSelector{},
			matches:  []labels.Set{prodWeb, devWeb, prodDB, noEnv, empty},
		},
		{
			name:     "matchLabels",
			selector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			matches:  []labels.Set{prodWeb, prodDB},
			rejects:  []labels.Set{devWeb, noEnv, empty},
		},
		{
			name: "In",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod", "staging"}},
			}},
			matches: []labels.Set{prodWeb, prodDB},
			rejects: []labels.Set{devWeb, noEnv, empty},
		},
		{
			name: "NotIn also matches monitors without the key",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"prod"}},
			}},
			matches: []labels.Set{devWeb, noEnv, empty},
			rejects: []labels.Set{prodWeb, prodDB},
		},
		{
			name: "Exists",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "env", Operator: metav1.LabelSelectorOpExists},
			}},
			matches: []labels.Set{prodWeb, devWeb, prodDB},
			rejects: []labels.Set{noEnv, empty},
		},
		{
			name: "DoesNotExist",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "env", Operator: metav1.LabelSelectorOpDoesNotExist},
			}},
			matches: []labels.Set{noEnv, empty},
			rejects: []labels.Set{prodWeb, devWeb, prodDB},
		},
		{
			name: "matchLabels and matchExpressions are ANDed",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web"}},
				},
			},
			matches: []labels.Set{prodWeb},
			rejects: []labels.Set{devWeb, prodDB, noEnv, empty},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel, err := getSelector(tc.selector)
			require.NoError(t, err)
			for _, set := range tc.matches {
				assert.Truef(t, sel.Matches(set), "selector %q should match %v", sel, set)
			}
			for _, set := range tc.rejects {
				assert.Falsef(t, sel.Matches(set), "selector %q should not match %v", sel, set)
			}
		})
	}
}

func TestGetSelectorRejectsInvalid(t *testing.T) {
	cases := []struct {
		name     string
		selector *metav1.LabelSelector
	}{
		{"unknown operator", &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: "Equals", Values: []string{"prod"}},
		}}},
		{"In with no values", &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: metav1.LabelSelectorOpIn},
		}}},
		{"NotIn with no values", &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: metav1.LabelSelectorOpNotIn},
		}}},
		{"Exists with values", &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: metav1.LabelSelectorOpExists, Values: []string{"prod"}},
		}}},
		{"invalid label key", &metav1.LabelSelector{MatchLabels: map[string]string{"not a valid key": "x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel, err := getSelector(tc.selector)
			assert.Error(t, err)
			assert.Nil(t, sel)
		})
	}
}

// TestNewPrometheusCRWatcherRejectsInvalidSelector checks that an invalid
// selector fails watcher construction at startup instead of matching all.
func TestNewPrometheusCRWatcherRejectsInvalidSelector(t *testing.T) {
	invalid := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "env", Operator: metav1.LabelSelectorOpIn},
	}}
	for _, tc := range []struct {
		name    string
		mutate  func(*allocatorconfig.Config)
		wantErr string
	}{
		{"service monitor selector", func(c *allocatorconfig.Config) { c.ServiceMonitorSelector = invalid }, "service_monitor_selector"},
		{"pod monitor selector", func(c *allocatorconfig.Config) { c.PodMonitorSelector = invalid }, "pod_monitor_selector"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTELCOL_NAMESPACE", "selector-ns")
			cfg := allocatorconfig.Config{
				ClusterConfig: &rest.Config{Host: "https://127.0.0.1:65535"},
				PrometheusCR: allocatorconfig.PrometheusCRConfig{
					Enabled:        true,
					ScrapeInterval: model.Duration(30 * time.Second),
				},
			}
			tc.mutate(&cfg)
			w, err := NewPrometheusCRWatcher(logr.Discard(), cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Nil(t, w)
		})
	}
}

type selectorTestMonitor struct {
	name      string
	labels    map[string]string
	annotated bool
}

// selectorTestMonitors cover each combination of the selector used in the
// LoadConfig tests below (env=prod AND tier In [web, api]) and the scraper
// annotation.
var selectorTestMonitors = []selectorTestMonitor{
	{name: "prod-web", labels: map[string]string{"env": "prod", "tier": "web"}},
	{name: "prod-api-routed", labels: map[string]string{"env": "prod", "tier": "api"}, annotated: true},
	{name: "prod-db", labels: map[string]string{"env": "prod", "tier": "db"}},
	{name: "dev-web", labels: map[string]string{"env": "dev", "tier": "web"}},
	{name: "dev-web-routed", labels: map[string]string{"env": "dev", "tier": "web"}, annotated: true},
	{name: "unlabelled"},
}

var prodWebOrAPI = &metav1.LabelSelector{
	MatchLabels: map[string]string{"env": "prod"},
	MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web", "api"}},
	},
}

// loadSelectedMonitors creates the test monitors of the given kind, applies the
// selector and scraper role, runs LoadConfig and returns the sorted names of
// the monitors that produced scrape jobs.
func loadSelectedMonitors(t *testing.T, kind string, selector *metav1.LabelSelector, scraperRole string) []string {
	t.Helper()
	w := getTestPrometheusCRWatcher(t, nil, nil)
	w.logger = logr.Discard()
	w.scraperRole = scraperRole
	defer func() { _ = w.Close() }()

	sel, err := getSelector(selector)
	require.NoError(t, err)
	if kind == "ServiceMonitor" {
		w.serviceMonitorSelector = sel
	} else {
		w.podMonitorSelector = sel
	}

	ctx := context.Background()
	for _, m := range selectorTestMonitors {
		meta := metav1.ObjectMeta{Name: m.name, Namespace: "test", Labels: m.labels}
		if m.annotated {
			meta.Annotations = map[string]string{ScraperAnnotationKey: clusterScraperRole}
		}
		if kind == "ServiceMonitor" {
			_, err = w.kubeMonitoringClient.MonitoringV1().ServiceMonitors("test").Create(ctx, &monitoringv1.ServiceMonitor{
				ObjectMeta: meta,
				Spec:       monitoringv1.ServiceMonitorSpec{Endpoints: []monitoringv1.Endpoint{{Port: "metrics"}}},
			}, metav1.CreateOptions{})
		} else {
			_, err = w.kubeMonitoringClient.MonitoringV1().PodMonitors("test").Create(ctx, &monitoringv1.PodMonitor{
				ObjectMeta: meta,
				Spec:       monitoringv1.PodMonitorSpec{PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: ptr.To("metrics")}}},
			}, metav1.CreateOptions{})
		}
		require.NoError(t, err)
	}

	notifyEvents := make(chan struct{}, 2)
	_, err = w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents)
	require.NoError(t, err)
	_, err = w.startMonitorInformer(monitoringv1.PodMonitorName, notifyEvents)
	require.NoError(t, err)

	got, err := w.LoadConfig(ctx)
	require.NoError(t, err)

	// Job names look like serviceMonitor/test/<name>/0 or podMonitor/test/<name>/0.
	var names []string
	for _, sc := range got.ScrapeConfigs {
		parts := strings.Split(sc.JobName, "/")
		require.Lenf(t, parts, 4, "unexpected job name %q", sc.JobName)
		names = append(names, parts[2])
	}
	sort.Strings(names)
	return names
}

// TestLoadConfigLabelSelector runs the selector through the real LoadConfig
// path for both monitor kinds, and checks that it composes with the
// cloudwatch.aws.amazon.com/scraper annotation routing: a monitor is scraped
// only when it matches the selector AND belongs to the agent's scraper role.
func TestLoadConfigLabelSelector(t *testing.T) {
	cases := []struct {
		name     string
		selector *metav1.LabelSelector
		role     string
		want     []string
	}{
		{
			name: "nil selector, default role: every unannotated monitor",
			want: []string{"dev-web", "prod-db", "prod-web", "unlabelled"},
		},
		{
			name: "nil selector, cluster-scraper role: every annotated monitor",
			role: clusterScraperRole,
			want: []string{"dev-web-routed", "prod-api-routed"},
		},
		{
			name:     "selector, default role: matching and unannotated",
			selector: prodWebOrAPI,
			want:     []string{"prod-web"},
		},
		{
			name:     "selector, cluster-scraper role: matching and annotated",
			selector: prodWebOrAPI,
			role:     clusterScraperRole,
			want:     []string{"prod-api-routed"},
		},
		{
			name: "DoesNotExist, default role",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "env", Operator: metav1.LabelSelectorOpDoesNotExist},
			}},
			want: []string{"unlabelled"},
		},
	}
	for _, kind := range []string{"ServiceMonitor", "PodMonitor"} {
		for _, tc := range cases {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				got := loadSelectedMonitors(t, kind, tc.selector, tc.role)
				assert.Equal(t, tc.want, got)
			})
		}
	}
}
