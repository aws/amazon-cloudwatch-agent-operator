// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectAnnotationKey(t *testing.T) {
	testCases := []struct {
		instType Type
		want     string
	}{
		{instType: TypeJava, want: annotationInjectJava},
		{instType: TypeNodeJS, want: annotationInjectNodeJS},
		{instType: TypePython, want: annotationInjectPython},
		{instType: TypeDotNet, want: annotationInjectDotNet},
		{instType: TypeGo, want: annotationInjectGo},
		{instType: "unsupported", want: ""},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, InjectAnnotationKey(testCase.instType))
	}
}

func TestTypeSet(t *testing.T) {
	types := []Type{TypeJava, TypeGo}
	ts := NewTypeSet(types...)
	_, ok := ts[TypeJava]
	assert.True(t, ok)
	_, ok = ts[TypeGo]
	assert.True(t, ok)
	_, ok = ts[TypePython]
	assert.False(t, ok)
}
