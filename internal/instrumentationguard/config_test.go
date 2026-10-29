// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMode(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   string
		want    Mode
		wantErr bool
	}{
		{name: "off", input: "off", want: ModeOff},
		{name: "dry-run", input: "dry-run", want: ModeDryRun},
		{name: "on is no longer a mode", input: "on", wantErr: true},
		{name: "unknown", input: "enabled", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "wrong case", input: "Off", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, ModeDryRun, cfg.Mode)
	assert.Equal(t, int32(3), cfg.RestartThreshold)
	assert.Equal(t, 10*time.Minute, cfg.Window)
	assert.Equal(t, 5*time.Minute, cfg.ImagePullPatience)
	assert.Equal(t, 30*time.Second, cfg.RequeueInterval)
	assert.NoError(t, cfg.Validate())
}

func TestConfigValidate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{name: "defaults", mutate: func(*Config) {}},
		{name: "mode off", mutate: func(c *Config) { c.Mode = ModeOff }},
		// The guard is detect-only, so there is no mode that writes to a workload. An operator
		// carrying the old flag value must fail to start rather than silently run in dry-run.
		{name: "mode on is no longer a mode", mutate: func(c *Config) { c.Mode = Mode("on") }, wantErr: true},
		{name: "unknown mode", mutate: func(c *Config) { c.Mode = Mode("sometimes") }, wantErr: true},
		{name: "threshold zero", mutate: func(c *Config) { c.RestartThreshold = 0 }, wantErr: true},
		{name: "threshold negative", mutate: func(c *Config) { c.RestartThreshold = -1 }, wantErr: true},
		{name: "threshold one", mutate: func(c *Config) { c.RestartThreshold = 1 }},
		{name: "zero window", mutate: func(c *Config) { c.Window = 0 }, wantErr: true},
		{name: "zero image pull patience", mutate: func(c *Config) { c.ImagePullPatience = 0 }, wantErr: true},
		{name: "negative image pull patience", mutate: func(c *Config) { c.ImagePullPatience = -time.Minute }, wantErr: true},
		{name: "image pull patience equal to the window", mutate: func(c *Config) { c.ImagePullPatience = c.Window }},
		{
			// A patience longer than the attribution window could never fire: the pod leaves the
			// window before the guard runs out of patience.
			name:    "image pull patience longer than the window",
			mutate:  func(c *Config) { c.ImagePullPatience = c.Window + time.Minute },
			wantErr: true,
		},
		{name: "zero requeue interval", mutate: func(c *Config) { c.RequeueInterval = 0 }, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
