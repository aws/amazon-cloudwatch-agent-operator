// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScrapeProtocolsDefaultedOnLoad checks that scrape_protocols is defaulted when the
// config is loaded, so the agent never receives an empty value.
func TestScrapeProtocolsDefaultedOnLoad(t *testing.T) {
	got := CreateDefaultConfig()
	err := LoadFromFile("./testdata/scrape_protocols_omitted_test.yaml", &got)
	require.NoError(t, err)

	require.NotNil(t, got.PromConfig)
	require.NotEmpty(t, got.PromConfig.ScrapeConfigs)

	for _, sc := range got.PromConfig.ScrapeConfigs {
		assert.NotEmpty(t, sc.ScrapeProtocols,
			"scrape_protocols must be defaulted for job %q", sc.JobName)
	}

	// Spot-check the static job from the fixture by name.
	found := false
	for _, sc := range got.PromConfig.ScrapeConfigs {
		if sc.JobName == "prometheus-sample-app" {
			found = true
			assert.Greater(t, len(sc.ScrapeProtocols), 0)
		}
	}
	require.True(t, found, "expected the 'prometheus-sample-app' job to be present")
}
