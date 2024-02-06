// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectCallbackFunc func(client.Object) bool

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
