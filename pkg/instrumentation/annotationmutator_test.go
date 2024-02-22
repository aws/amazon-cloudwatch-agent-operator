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
		"TestInsert/Conflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
					"keyC": "4",
				}),
			},
			wantAnnotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			wantMutated: false,
		},
		"TestInsert/NoConflicts": {
			annotations: nil,
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
					"keyC": "4",
				}),
			},
			wantAnnotations: map[string]string{
				"keyA": "3",
				"keyC": "4",
			},
			wantMutated: true,
		},
		"TestInsert/Multiple": {
			annotations: nil,
			mutations: []AnnotationMutation{
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
				}),
				NewInsertAnnotationMutation(map[string]string{
					"keyC": "4",
				}),
			},
			wantAnnotations: map[string]string{
				"keyA": "3",
				"keyC": "4",
			},
			wantMutated: true,
		},
		"TestRemove/Conflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
					"keyC",
				}),
			},
			wantAnnotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			wantMutated: false,
		},
		"TestRemove/NoConflicts": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
					"keyB",
				}),
			},
			wantAnnotations: map[string]string{},
			wantMutated:     true,
		},
		"TestRemove/Multiple": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
				}),
				NewRemoveAnnotationMutation([]string{
					"keyB",
				}),
			},
			wantAnnotations: map[string]string{},
			wantMutated:     true,
		},
		"TestBoth": {
			annotations: map[string]string{
				"keyA": "1",
				"keyB": "2",
			},
			mutations: []AnnotationMutation{
				NewRemoveAnnotationMutation([]string{
					"keyA",
				}),
				NewInsertAnnotationMutation(map[string]string{
					"keyA": "3",
				}),
			},
			wantAnnotations: map[string]string{
				"keyA": "3",
				"keyB": "2",
			},
			wantMutated: true,
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
