// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifests

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Builder func(params Params) ([]client.Object, error)

type ManifestFactory[T client.Object] func(params Params) (T, error)
type SimpleManifestFactory[T client.Object] func(params Params) T
type K8sManifestFactory ManifestFactory[client.Object]

func FactoryWithoutError[T client.Object](f SimpleManifestFactory[T]) K8sManifestFactory {
	return func(params Params) (client.Object, error) {
		return f(params), nil
	}
}

func Factory[T client.Object](f ManifestFactory[T]) K8sManifestFactory {
	return func(params Params) (client.Object, error) {
		return f(params)
	}
}

// ObjectIsNotNil ensures that we only create an object IFF it isn't nil,
// and it's concrete type isn't nil either. This works around the Go type system
// by using reflection to verify its concrete type isn't nil.
func ObjectIsNotNil(obj client.Object) bool {
	return obj != nil && !reflect.ValueOf(obj).IsNil()
}
