// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAnnotationRoleMatches verifies the annotation-based routing partition: the cluster-scraper
// role selects only monitors annotated cloudwatch.aws/scraper: cluster-scraper, and the default
// role (empty) selects only monitors that are not so annotated. The two roles are complementary,
// so every monitor is owned by exactly one role (no overlap, no gap).
func TestAnnotationRoleMatches(t *testing.T) {
	routed := map[string]string{ScraperAnnotationKey: clusterScraperRole}
	other := map[string]string{ScraperAnnotationKey: "something-else"}
	none := map[string]string{"unrelated": "x"}

	cases := []struct {
		name        string
		role        string
		annotations map[string]string
		want        bool
	}{
		{"cluster-scraper claims routed", clusterScraperRole, routed, true},
		{"cluster-scraper skips unannotated", clusterScraperRole, none, false},
		{"cluster-scraper skips other value", clusterScraperRole, other, false},
		{"cluster-scraper skips nil", clusterScraperRole, nil, false},
		{"default claims unannotated", "", none, true},
		{"default claims nil", "", nil, true},
		{"default claims other value", "", other, true},
		{"default skips routed", "", routed, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := annotationRoleMatches(tc.role, tc.annotations); got != tc.want {
				t.Fatalf("annotationRoleMatches(%q, %v) = %v, want %v", tc.role, tc.annotations, got, tc.want)
			}
		})
	}

	// Partition invariant: for any monitor, exactly one role claims it.
	for _, ann := range []map[string]string{routed, other, none, nil} {
		cs := annotationRoleMatches(clusterScraperRole, ann)
		def := annotationRoleMatches("", ann)
		if cs == def {
			t.Fatalf("partition broken for %v: cluster-scraper=%v default=%v (must differ)", ann, cs, def)
		}
	}
}

// TestSelectsMonitor covers PrometheusCRWatcher.selectsMonitor for both scraper roles, and asserts
// the cluster-scraper agent logs each monitor it claims (the override event) while the per-node
// agent logs nothing.
func TestSelectsMonitor(t *testing.T) {
	routed := map[string]string{ScraperAnnotationKey: clusterScraperRole}
	none := map[string]string{}

	cases := []struct {
		name        string
		role        string
		annotations map[string]string
		wantSelect  bool
		wantLog     bool
	}{
		{"cluster-scraper claims routed (logs override)", clusterScraperRole, routed, true, true},
		{"cluster-scraper skips unannotated", clusterScraperRole, none, false, false},
		{"default claims unannotated (no log)", "", none, true, false},
		{"default skips routed", "", routed, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs []string
			logger := funcr.New(func(prefix, args string) { logs = append(logs, args) }, funcr.Options{})
			w := &PrometheusCRWatcher{logger: logger, scraperRole: tc.role}

			got := w.selectsMonitor("PodMonitor", "ns", "mon", tc.annotations)
			assert.Equal(t, tc.wantSelect, got, "selectsMonitor result")

			logged := strings.Join(logs, "\n")
			if tc.wantLog {
				assert.Contains(t, logged, "cluster-scraper", "expected an override log mentioning cluster-scraper")
				assert.Contains(t, logged, "mon", "override log should identify the monitor")
			} else {
				assert.Empty(t, logs, "no override log expected for role=%q select=%v", tc.role, got)
			}
		})
	}
}

// TestLoadConfigScraperRouting exercises the annotation filter through the real LoadConfig path
// (matching TestLoadConfig's harness): an annotated ServiceMonitor is discovered by the
// cluster-scraper role and excluded by the default role.
func TestLoadConfigScraperRouting(t *testing.T) {
	newAnnotatedSM := func() *monitoringv1.ServiceMonitor {
		return &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "routed",
				Namespace:   "test",
				Annotations: map[string]string{ScraperAnnotationKey: clusterScraperRole},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{{Port: "metrics"}},
			},
		}
	}

	cases := []struct {
		name         string
		role         string
		wantSelected bool
	}{
		{"cluster-scraper claims annotated monitor", clusterScraperRole, true},
		{"default role excludes annotated monitor", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := getTestPrometheusCRWatcher(t, newAnnotatedSM(), nil)
			w.logger = logr.Discard()
			w.scraperRole = tc.role
			for _, informer := range w.informers {
				informer.Start(w.stopChannel)
			}
			for _, informer := range w.informers {
				for !informer.HasSynced() {
					time.Sleep(50 * time.Millisecond)
				}
			}

			got, err := w.LoadConfig(context.Background())
			require.NoError(t, err)

			var found bool
			for _, sc := range got.ScrapeConfigs {
				if strings.Contains(sc.JobName, "routed") {
					found = true
					break
				}
			}
			assert.Equalf(t, tc.wantSelected, found,
				"annotated monitor discovered=%v, want %v for scraperRole=%q", found, tc.wantSelected, tc.role)
		})
	}
}
