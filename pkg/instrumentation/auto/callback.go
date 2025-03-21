// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectCallbackFunc func(client.Object, any) (any, bool)

// chainCallbacks is a func that invokes functions in a callback chain one after another as long as each function
// returns true. The result of each function in the callback is passed along to the next. Eventually returns true if
// all callbacks in chain are executed and false otherwise.
func chainCallbacks(fns ...objectCallbackFunc) objectCallbackFunc {
	return func(obj client.Object, passToNext any) (any, bool) {
		var ok bool
		for _, fn := range fns {
			passToNext, ok = fn(obj, passToNext)
			if !ok {
				return passToNext, false
			}
		}
		return passToNext, true
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

func patchFunc(m InstrumentationAnnotator, ctx context.Context, callback objectCallbackFunc) objectCallbackFunc {
	return func(obj client.Object, _ any) (any, bool) {
		patch, err := createPatch(obj)
		if err != nil {
			m.GetLogger().Error(err, "Unable to create patch",
				"kind", fmt.Sprintf("%T", obj),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			return nil, false
		}
		ret, ok := callback(obj, nil)
		if !ok {
			return ret, false
		}
		if err = m.GetWriter().Patch(ctx, obj, patch); err != nil {
			m.GetLogger().Error(err, "Unable to send patch",
				"kind", fmt.Sprintf("%T", obj),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			return ret, false
		}
		return ret, true
	}
}

func restartNamespaceFunc(m InstrumentationAnnotator, ctx context.Context) objectCallbackFunc {
	return func(obj client.Object, previousResult any) (any, bool) {
		mutatedAnnotations, ok := previousResult.(map[string]string)
		if !ok {
			return nil, false
		}
		namespace, ok := obj.(*corev1.Namespace)
		if !ok {
			return nil, false
		}
		RestartNamespace(m, ctx, namespace, mutatedAnnotations)
		return nil, true
	}
}

// shouldRestartFunc returns a func that determines if a resource should be restarted
func shouldRestartFunc(m InstrumentationAnnotator, namespaceMutatedAnnotations map[string]string) objectCallbackFunc {
	return func(obj client.Object, _ any) (any, bool) {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			return nil, shouldRestartResource(m, namespaceMutatedAnnotations, o.Spec.Template.GetObjectMeta())
		case *appsv1.DaemonSet:
			return nil, shouldRestartResource(m, namespaceMutatedAnnotations, o.Spec.Template.GetObjectMeta())
		case *appsv1.StatefulSet:
			return nil, shouldRestartResource(m, namespaceMutatedAnnotations, o.Spec.Template.GetObjectMeta())
		default:
			return nil, false
		}
	}
}

// shouldRestartResource returns true if a resource requires a restart corresponding to the mutated annotations on its namespace
func shouldRestartResource(m InstrumentationAnnotator, namespaceMutatedAnnotations map[string]string, obj metav1.Object) bool {
	var shouldRestart bool

	if resourceAnnotations := obj.GetAnnotations(); resourceAnnotations != nil {
		// For each of the namespace mutated annotations,
		for namespaceMutatedAnnotation, namespaceMutatedAnnotationValue := range namespaceMutatedAnnotations {
			if _, ok := m.GetAnnotationMutators().injectAnnotations[namespaceMutatedAnnotation]; !ok {
				// If it is not an inject-* annotation, we can ignore it
				continue
			}
			resourceAnnotationValue, ok := resourceAnnotations[namespaceMutatedAnnotation]
			if ok && namespaceMutatedAnnotationValue == resourceAnnotationValue {
				// If the resource already has the same annotation with the same value, do not restart it since it
				// was explicitly annotated on the resource and hence the annotation on the namespace being mutated
				// should have no overall impact
				continue
			} else {
				// Else the resource needs to be instrumented/un-instrumented via the namespace and hence needs a restart
				shouldRestart = true
			}
		}
	} else {
		shouldRestart = true
	}

	return shouldRestart
}

// RestartNamespace sets the restartedAtAnnotation for each of the namespace's supported resources and patches them.
func RestartNamespace(m InstrumentationAnnotator, ctx context.Context, namespace *corev1.Namespace, mutatedAnnotations map[string]string) {
	callbackFunc := patchFunc(m, ctx, setRestartAnnotation)
	rangeObjectList(m, ctx, &appsv1.DeploymentList{}, client.InNamespace(namespace.Name), chainCallbacks(shouldRestartFunc(m, mutatedAnnotations), callbackFunc))
	rangeObjectList(m, ctx, &appsv1.DaemonSetList{}, client.InNamespace(namespace.Name), chainCallbacks(shouldRestartFunc(m, mutatedAnnotations), callbackFunc))
	rangeObjectList(m, ctx, &appsv1.StatefulSetList{}, client.InNamespace(namespace.Name), chainCallbacks(shouldRestartFunc(m, mutatedAnnotations), callbackFunc))
}

// MutateAndPatchAll runs the mutators for each of the supported resources and patches them.
func MutateAndPatchAll(m InstrumentationAnnotator, ctx context.Context) {
	f := getMutateObjectFunc(m)
	callbackFunc := patchFunc(m, ctx, f)
	rangeObjectList(m, ctx, &appsv1.DeploymentList{}, &client.ListOptions{}, callbackFunc)
	rangeObjectList(m, ctx, &appsv1.DaemonSetList{}, &client.ListOptions{}, callbackFunc)
	rangeObjectList(m, ctx, &appsv1.StatefulSetList{}, &client.ListOptions{}, callbackFunc)
	rangeObjectList(m, ctx, &corev1.NamespaceList{}, &client.ListOptions{}, chainCallbacks(callbackFunc, restartNamespaceFunc(m, ctx)))
}

func getMutateObjectFunc(m InstrumentationAnnotator) objectCallbackFunc {
	return func(obj client.Object, _ any) (any, bool) {
		return m.MutateObject(nil, obj), true
	}
}
