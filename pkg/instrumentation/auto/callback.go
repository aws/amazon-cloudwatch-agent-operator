// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectCallbackFunc func(client.Object) bool

// chainCallbacks is a func that invokes functions in a callback chain one after another as long as each function
// returns true. Eventually returns true if all callbacks in chain are executed and false otherwise.
func chainCallbacks(fns ...objectCallbackFunc) objectCallbackFunc {
	return func(obj client.Object) bool {
		for _, fn := range fns {
			if !fn(obj) {
				return false
			}
		}
		return true
	}
}

// basicPatch based on client.mergeFromPatch. Takes in a pre-marshalled JSON instead of the original object.
type basicPatch struct {
	originalJSON []byte
}

var _ client.Patch = (*basicPatch)(nil)

func (p *basicPatch) Type() types.PatchType {
	return types.MergePatchType
}

func (p *basicPatch) Data(obj client.Object) ([]byte, error) {
	modifiedJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	data, err := strategicpatch.CreateTwoWayMergePatch(p.originalJSON, modifiedJSON, obj)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func createPatch(obj client.Object) (client.Patch, error) {
	originalJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return &basicPatch{originalJSON: originalJSON}, nil
}

func (m *AnnotationMutators) patchFunc(ctx context.Context, callback objectCallbackFunc) objectCallbackFunc {
	return func(obj client.Object) bool {
		patch, err := createPatch(obj)
		if err != nil {
			m.logger.Error(err, "Unable to create patch",
				"kind", fmt.Sprintf("%T", obj),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			return false
		}
		if !callback(obj) {
			return false
		}
		if err = m.clientWriter.Patch(ctx, obj, patch); err != nil {
			m.logger.Error(err, "Unable to send patch",
				"kind", fmt.Sprintf("%T", obj),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			return false
		}
		return true
	}
}

func (m *AnnotationMutators) restartNamespaceFunc(ctx context.Context) objectCallbackFunc {
	return func(obj client.Object) bool {
		namespace, ok := obj.(*corev1.Namespace)
		if !ok {
			return false
		}
		m.RestartNamespace(ctx, namespace)
		return true
	}
}

// checkDifferentInjectAnnotationsFunc returns a func that checks if a namespace and an object have the same set of
// inject annotations
func (m *AnnotationMutators) checkDifferentInjectAnnotationsFunc(namespace *corev1.Namespace) objectCallbackFunc {
	return func(obj client.Object) bool {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			return m.checkDifferentInjectAnnotations(namespace.GetObjectMeta(), o.Spec.Template.GetObjectMeta())
		case *appsv1.DaemonSet:
			return m.checkDifferentInjectAnnotations(namespace.GetObjectMeta(), o.Spec.Template.GetObjectMeta())
		case *appsv1.StatefulSet:
			return m.checkDifferentInjectAnnotations(namespace.GetObjectMeta(), o.Spec.Template.GetObjectMeta())
		default:
			return false
		}
	}
}

// checkDifferentInjectAnnotations returns true if both the objects do NOT have identical inject annotations and false
// otherwise. If both objects do not have any inject annotations, return false.
func (m *AnnotationMutators) checkDifferentInjectAnnotations(obj1, obj2 metav1.Object) bool {
	obj1InjectAnnotations := make(map[string]string)
	if annotations := obj1.GetAnnotations(); annotations != nil {
		for annotation, value := range annotations {
			if _, ok := m.injectAnnotations[annotation]; ok {
				obj1InjectAnnotations[annotation] = value
			}
		}
	}
	obj2InjectAnnotations := make(map[string]string)
	if annotations := obj2.GetAnnotations(); annotations != nil {
		for annotation, value := range annotations {
			if _, ok := m.injectAnnotations[annotation]; ok {
				obj2InjectAnnotations[annotation] = value
			}
		}
	}
	return !reflect.DeepEqual(obj1InjectAnnotations, obj2InjectAnnotations)
}
