// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package receiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkywalkingSelfRegisters(t *testing.T) {
	// verify
	assert.True(t, IsRegistered("skywalking"))
}

func TestSkywalkingIsFoundByName(t *testing.T) {
	// test
	p, err := For(logger, "skywalking", map[interface{}]interface{}{})
	assert.NoError(t, err)

	// verify
	assert.Equal(t, "__skywalking", p.ParserName())
}

func TestSkywalkingPortsOverridden(t *testing.T) {
	// prepare
	builder := NewSkywalkingReceiverParser(logger, "skywalking", map[interface{}]interface{}{
		"protocols": map[interface{}]interface{}{
			"grpc": map[interface{}]interface{}{
				"endpoint": "0.0.0.0:1234",
			},
			"http": map[interface{}]interface{}{
				"endpoint": "0.0.0.0:1235",
			},
		},
	})

	expectedResults := map[string]struct {
		portNumber int32
		seen       bool
	}{
		"skywalking-grpc": {portNumber: 1234},
		"skywalking-http": {portNumber: 1235},
	}

	// test
	ports, err := builder.Ports()

	// verify
	assert.NoError(t, err)
	assert.Len(t, ports, len(expectedResults))

	for _, port := range ports {
		r := expectedResults[port.Name]
		r.seen = true
		expectedResults[port.Name] = r
		assert.EqualValues(t, r.portNumber, port.Port)
	}
	for k, v := range expectedResults {
		assert.True(t, v.seen, "the port %s wasn't included in the service ports", k)
	}
}

func TestSkywalkingExposeDefaultPorts(t *testing.T) {
	// prepare
	builder := NewSkywalkingReceiverParser(logger, "skywalking", map[interface{}]interface{}{
		"protocols": map[interface{}]interface{}{
			"grpc": map[interface{}]interface{}{},
			"http": map[interface{}]interface{}{},
		},
	})

	expectedResults := map[string]struct {
		portNumber int32
		seen       bool
	}{
		"skywalking-grpc": {portNumber: 11800},
		"skywalking-http": {portNumber: 12800},
	}

	// test
	ports, err := builder.Ports()

	// verify
	assert.NoError(t, err)
	assert.Len(t, ports, len(expectedResults))

	for _, port := range ports {
		r := expectedResults[port.Name]
		r.seen = true
		expectedResults[port.Name] = r
		assert.EqualValues(t, r.portNumber, port.Port)
	}
	for k, v := range expectedResults {
		assert.True(t, v.seen, "the port %s wasn't included in the service ports", k)
	}
}
