// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import "testing"

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
