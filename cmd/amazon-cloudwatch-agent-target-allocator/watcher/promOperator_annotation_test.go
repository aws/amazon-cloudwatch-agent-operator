// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"testing"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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


// TestLoadConfigAnnotationRouting verifies the routing filter is actually applied
// during monitor discovery in LoadConfig: a Target Allocator only emits scrape
// jobs for the monitors its scraperRole owns, and skips the rest.
func TestLoadConfigAnnotationRouting(t *testing.T) {
	annotated := map[string]string{ScraperAnnotationKey: clusterScraperRole}

	sm := func(ann map[string]string) *monitoringv1.ServiceMonitor {
		return &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: "simple", Namespace: "test", Annotations: ann},
			Spec: monitoringv1.ServiceMonitorSpec{
				JobLabel:  "test",
				Endpoints: []monitoringv1.Endpoint{{Port: "web"}},
			},
		}
	}
	pm := func(ann map[string]string) *monitoringv1.PodMonitor {
		return &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: "simple", Namespace: "test", Annotations: ann},
			Spec: monitoringv1.PodMonitorSpec{
				JobLabel:            "test",
				PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: ptr.To("web")}},
			},
		}
	}

	tests := []struct {
		name     string
		role     string
		smAnn    map[string]string
		pmAnn    map[string]string
		wantJobs int
	}{
		{"default role skips cluster-scraper monitors", "", annotated, annotated, 0},
		{"cluster-scraper role skips unannotated monitors", clusterScraperRole, nil, nil, 0},
		{"cluster-scraper role keeps annotated monitors", clusterScraperRole, annotated, annotated, 2},
		{"default role keeps unannotated monitors", "", nil, nil, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := getTestPrometheusCRWatcher(t, sm(tt.smAnn), pm(tt.pmAnn))
			w.scraperRole = tt.role
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
			assert.Len(t, got.ScrapeConfigs, tt.wantJobs)
		})
	}
}
