// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"encoding/json"
)

// Type is an enum for instrumentation types.
type Type string

// TypeSet is a map with Type keys.
type TypeSet map[Type]any

func (s *TypeSet) UnmarshalJSON(data []byte) error {
	var types []Type
	if err := json.Unmarshal(data, &types); err != nil {
		return err
	}
	*s = NewTypeSet(types...)
	return nil
}

func (s TypeSet) MarshalJSON() ([]byte, error) {
	var types []Type
	for t := range s {
		types = append(types, t)
	}
	return json.Marshal(types)
}

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

func SupportedTypes() []Type {
	return []Type{TypeJava, TypeNodeJS, TypePython, TypeDotNet}
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
