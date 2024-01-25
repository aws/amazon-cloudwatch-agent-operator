// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutateAnnotations(t *testing.T) {
	testCases := map[string]struct {
		annotations     map[string]string
		mutations       []AnnotationMutation
		wantAnnotations map[string]string
		wantMutated     bool
	}{
		"TestInsert/Any": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
					"keyC": "4",
				}, false),
			},
			wantAnnotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
				"keyC": "4",
			},
			wantMutated: true,
		},
		"TestInsert/All/Conflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
					"keyC": "4",
				}, true),
			},
			wantAnnotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			wantMutated: false,
		},
		"TestInsert/All/NoConflicts": {
			annotations: nil,
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
					"keyC": "4",
				}, true),
			},
			wantAnnotations: map[string]string{
				"keyA": "3",
				"keyC": "4",
			},
			wantMutated: true,
		},
		"TestRemove/Any": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
					"keyC",
				}, false),
			},
			wantAnnotations: map[string]string{
				"keyB": "2",
			},
			wantMutated: true,
		},
		"TestRemove/All/Conflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
					"keyC",
				}, true),
			},
			wantAnnotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			wantMutated: false,
		},
		"TestRemove/All/NoConflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
					"keyB",
				}, true),
			},
			wantAnnotations: map[string]string{},
			wantMutated:     true,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := metav1.ObjectMeta{
				Annotations: testCase.annotations,
			}
			m := NewAnnotationMutator(testCase.mutations)
			assert.Equal(t, testCase.wantMutated, m.Mutate(&obj))
			assert.Equal(t, testCase.wantAnnotations, obj.GetAnnotations())
		})
	}
}
