// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

// Type is an enum for instrumentation types.
type Type string

// TypeSet is a map with Type keys.
type TypeSet map[Type]any

// NewTypeSet creates a new set of Type.
func NewTypeSet(types ...Type) TypeSet {
	s := make(TypeSet, len(types))
	for _, t := range types {
		s[t] = nil
	}
	return s
}

const (
	TypeJava   Type = "java"
	TypeNodeJS Type = "nodejs"
	TypePython Type = "python"
	TypeDotNet Type = "dotnet"
	TypeGo     Type = "go"
)

func AllTypes() []Type {
	return []Type{TypeJava, TypeNodeJS, TypePython, TypeDotNet, TypeGo}
}

// InjectAnnotationKey maps the instrumentation type to the inject annotation.
func InjectAnnotationKey(instType Type) string {
	switch instType {
	case TypeJava:
		return annotationInjectJava
	case TypeNodeJS:
		return annotationInjectNodeJS
	case TypePython:
		return annotationInjectPython
	case TypeDotNet:
		return annotationInjectDotNet
	case TypeGo:
		return annotationInjectGo
	default:
		return ""
	}
}
