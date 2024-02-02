// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
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

func (m *AnnotationMutators) updateFunc(ctx context.Context) objectCallbackFunc {
	return func(obj client.Object) bool {
		if err := m.clientWriter.Update(ctx, obj); err != nil {
			m.logger.Error(err, "Unable to send update",
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
	restartAndUpdateFunc := chainCallbacks(restart, m.updateFunc(ctx))
	return func(obj client.Object) bool {
		namespace, ok := obj.(*corev1.Namespace)
		if !ok {
			return false
		}
		m.rangeObjectList(ctx, &appsv1.DeploymentList{}, client.InNamespace(namespace.Name), restartAndUpdateFunc)
		m.rangeObjectList(ctx, &appsv1.DaemonSetList{}, client.InNamespace(namespace.Name), restartAndUpdateFunc)
		m.rangeObjectList(ctx, &appsv1.StatefulSetList{}, client.InNamespace(namespace.Name), restartAndUpdateFunc)
		return true
	}
}
