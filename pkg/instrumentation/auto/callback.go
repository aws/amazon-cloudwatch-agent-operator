// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
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

func (m *AnnotationMutators) patchFunc(ctx context.Context, callback objectCallbackFunc) objectCallbackFunc {
	return func(obj client.Object) bool {
		patch := client.StrategicMergeFrom(obj.DeepCopyObject().(client.Object))
		if !callback(obj) {
			return false
		}
		if err := m.clientWriter.Patch(ctx, obj, patch); err != nil {
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
