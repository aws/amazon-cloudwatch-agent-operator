// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.47.0",
	}

	t.Run("should return expected cwagent config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "amazon-cloudwatch-agent"
		expectedLables["app.kubernetes.io/name"] = "test-agent"
		expectedLables["app.kubernetes.io/version"] = "0.0.0"

		expectedData := map[string]string{
			"cwagentconfig.json": `{"logs":{"metrics_collected":{"application_signals":{},"kubernetes":{"enhanced_container_insights":true}}},"traces":{"traces_collected":{"application_signals":{}}}}`,
		}

		param := deploymentParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "test-agent", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}
