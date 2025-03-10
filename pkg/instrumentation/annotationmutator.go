// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AnnotationMutation is used to modify an annotation map.
type AnnotationMutation interface {
	// Mutate attempts to modify the annotations map. Returns the mutated annotations if any.
	Mutate(annotations map[string]string) map[string]string
}

type insertAnnotationMutation struct {
	insert map[string]string
}

func (m *insertAnnotationMutation) Mutate(annotations map[string]string) map[string]string {
	mutatedAnnotations := make(map[string]string)
	for key, value := range m.insert {
		if _, ok := annotations[key]; !ok {
			annotations[key] = value
			mutatedAnnotations[key] = value
		}
	}
	return mutatedAnnotations
}

// NewInsertAnnotationMutation creates a new mutation that inserts annotations. Any missing annotation key
// is sufficient for it to attempt to insert.
func NewInsertAnnotationMutation(annotations map[string]string) AnnotationMutation {
	return &insertAnnotationMutation{insert: annotations}
}

type removeAnnotationMutation struct {
	remove []string
}

func (m *removeAnnotationMutation) Mutate(annotations map[string]string) map[string]string {
	mutatedAnnotations := make(map[string]string)
	if !m.shouldMutate(annotations) {
		return mutatedAnnotations
	}
	for _, key := range m.remove {
		if value, ok := annotations[key]; ok {
			delete(annotations, key)
			mutatedAnnotations[key] = value
		}
	}
	return mutatedAnnotations
}

func (m *removeAnnotationMutation) shouldMutate(annotations map[string]string) bool {
	for _, key := range m.remove {
		if _, ok := annotations[key]; !ok {
			return false
		}
	}
	return true
}

// NewRemoveAnnotationMutation creates a new mutation that removes annotations. All provided annotation keys
// must be present for it to attempt to remove them.
func NewRemoveAnnotationMutation(annotations []string) AnnotationMutation {
	return &removeAnnotationMutation{remove: annotations}
}

type AnnotationMutator struct {
	mutations []AnnotationMutation
}

// NewAnnotationMutator creates a mutator with the provided mutations that can mutate an Object's annotations.
func NewAnnotationMutator(mutations []AnnotationMutation) AnnotationMutator {
	return AnnotationMutator{mutations: mutations}
}

// Mutate modifies the object's annotations based on the mutator's mutations. Returns all the mutated annotations.
func (m AnnotationMutator) Mutate(obj metav1.Object) map[string]string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	allMutatedAnnotations := make(map[string]string)
	for _, mutation := range m.mutations {
		mutatedAnnotations := mutation.Mutate(annotations)
		for k, v := range mutatedAnnotations {
			allMutatedAnnotations[k] = v
		}
	}
	obj.SetAnnotations(annotations)
	return allMutatedAnnotations
}

// ObjectAnnotationMutator responsible for getting, mutating, and setting the object's annotations map. Returns all the mutated annotations.
type ObjectAnnotationMutator interface {
	Mutate(obj metav1.Object) map[string]string
}
