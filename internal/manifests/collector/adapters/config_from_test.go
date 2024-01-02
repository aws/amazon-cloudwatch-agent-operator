// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package adapters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

func TestInvalidYAML(t *testing.T) {
	// test
	config, err := adapters.ConfigFromString("🦄")

	// verify
	assert.Nil(t, config)
	assert.Equal(t, adapters.ErrInvalidYAML, err)
}

func TestEmptyString(t *testing.T) {
	// test and verify
	res, err := adapters.ConfigFromString("")
	assert.NoError(t, err)
	assert.Empty(t, res, 0)
}
