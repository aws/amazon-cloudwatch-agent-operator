// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package config contains the operator's runtime configuration.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeHandler(t *testing.T) {
	// prepare
	internal := 0
	callback := func() error {
		internal += 1
		return nil
	}
	h := newOnChange()

	h.Register(callback)

	for i := 0; i < 5; i++ {
		assert.Equal(t, i, internal)
		require.NoError(t, h.Do())
		assert.Equal(t, i+1, internal)
	}
}
