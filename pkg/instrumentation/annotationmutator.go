// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AnnotationMutation is used to modify an annotation map.
type AnnotationMutation interface {
	// Mutate attempts to modify the annotations map. Returns whether the function changed the annotations.
	Mutate(annotations map[string]string) bool
}

type insertAnnotationMutation struct {
	insert     map[string]string
	requireAll bool
}

func (m *insertAnnotationMutation) Mutate(annotations map[string]string) bool {
	if m.requireAll && !m.validate(annotations) {
		return false
	}
	var mutated bool
	for key, value := range m.insert {
		if _, ok := annotations[key]; !ok {
			annotations[key] = value
			mutated = true
		}
	}
	return mutated
}

func (m *insertAnnotationMutation) validate(annotations map[string]string) bool {
	for key := range m.insert {
		if _, ok := annotations[key]; ok {
			return false
		}
	}
	return true
}

// NewInsertAnnotationMutation creates a new mutation that inserts annotations. If requireAll is enabled,
// all provided annotation keys must be present for it to attempt to insert.
func NewInsertAnnotationMutation(annotations map[string]string, requireAll bool) AnnotationMutation {
	return &insertAnnotationMutation{insert: annotations, requireAll: requireAll}
}

type removeAnnotationMutation struct {
	remove     []string
	requireAll bool
}

func (m *removeAnnotationMutation) Mutate(annotations map[string]string) bool {
	if m.requireAll && !m.validate(annotations) {
		return false
	}
	var mutated bool
	for _, key := range m.remove {
		if _, ok := annotations[key]; ok {
			delete(annotations, key)
			mutated = true
		}
	}
	return mutated
}

func (m *removeAnnotationMutation) validate(annotations map[string]string) bool {
	for _, key := range m.remove {
		if _, ok := annotations[key]; !ok {
			return false
		}
	}
	return true
}

// NewRemoveAnnotationMutation creates a new mutation that removes annotations. If requireAll is enabled,
// all provided annotation keys must be present for it to attempt to remove them.
func NewRemoveAnnotationMutation(annotations []string, requireAll bool) AnnotationMutation {
	return &removeAnnotationMutation{remove: annotations, requireAll: requireAll}
}

type AnnotationMutator struct {
	mutations []AnnotationMutation
}

// NewAnnotationMutator creates a mutator with the provided mutations that can mutate an Object's annotations.
func NewAnnotationMutator(mutations []AnnotationMutation) AnnotationMutator {
	return AnnotationMutator{mutations: mutations}
}

// Mutate modifies the object's annotations based on the mutator's mutations. Returns whether any of the
// mutations changed the annotations.
func (m *AnnotationMutator) Mutate(obj metav1.Object) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	var mutated bool
	for _, mutation := range m.mutations {
		mutated = mutated || mutation.Mutate(annotations)
	}
	obj.SetAnnotations(annotations)
	return mutated
}
