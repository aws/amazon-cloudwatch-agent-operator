// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	var defaulAllocationStrategy = DefaultAllocationStrategy
	type args struct {
		file string
	}
	tests := []struct {
		name           string
		args           args
		wantErr        assert.ErrorAssertionFunc
		wantHTTPS      HTTPSServerConfig
		wantLabels     map[string]string
		wantPromCR     PrometheusCRConfig
		wantAlloc      *string
		wantPodMonSel  map[string]string
		wantSvcMonSel  map[string]string
		wantJobNames   []string
	}{
		{
			name: "file sd load",
			args: args{
				file: "./testdata/config_test.yaml",
			},
			wantErr: assert.NoError,
			wantHTTPS: HTTPSServerConfig{
				Enabled:         true,
				ListenAddr:      DefaultListenAddr,
				CAFilePath:      "/path/to/ca.pem",
				TLSCertFilePath: "/path/to/cert.pem",
				TLSKeyFilePath:  "/path/to/key.pem",
			},
			wantLabels: map[string]string{
				"app.kubernetes.io/instance":   "default.test",
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
			wantPromCR: PrometheusCRConfig{
				ScrapeInterval: model.Duration(time.Second * 60),
			},
			wantAlloc:    &defaulAllocationStrategy,
			wantJobNames: []string{"prometheus"},
		},
		{
			name: "no config",
			args: args{
				file: "./testdata/no_config.yaml",
			},
			wantErr:   assert.NoError,
			wantHTTPS: CreateDefaultConfig().HTTPS,
			wantLabels: nil,
			wantPromCR: CreateDefaultConfig().PrometheusCR,
			wantAlloc:  CreateDefaultConfig().AllocationStrategy,
		},
		{
			name: "service monitor pod monitor selector",
			args: args{
				file: "./testdata/pod_service_selector_test.yaml",
			},
			wantErr: assert.NoError,
			wantHTTPS: HTTPSServerConfig{
				Enabled:         true,
				ListenAddr:      DefaultListenAddr,
				CAFilePath:      DefaultCABundlePath,
				TLSCertFilePath: DefaultTLSCertPath,
				TLSKeyFilePath:  DefaultTLSKeyPath,
			},
			wantLabels: map[string]string{
				"app.kubernetes.io/instance":   "default.test",
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
			wantPromCR: PrometheusCRConfig{
				ScrapeInterval: DefaultCRScrapeInterval,
			},
			wantAlloc: &defaulAllocationStrategy,
			wantPodMonSel: map[string]string{
				"release": "test",
			},
			wantSvcMonSel: map[string]string{
				"release": "test",
			},
			wantJobNames: []string{"prometheus"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateDefaultConfig()
			err := LoadFromFile(tt.args.file, &got)
			if !tt.wantErr(t, err, fmt.Sprintf("Load(%v)", tt.args.file)) {
				return
			}
			assert.Equal(t, tt.wantHTTPS, got.HTTPS)
			assert.Equal(t, tt.wantLabels, got.LabelSelector)
			assert.Equal(t, tt.wantPromCR, got.PrometheusCR)
			assert.Equal(t, tt.wantAlloc, got.AllocationStrategy)
			assert.Equal(t, tt.wantPodMonSel, got.PodMonitorSelector)
			assert.Equal(t, tt.wantSvcMonSel, got.ServiceMonitorSelector)
			if tt.wantJobNames != nil {
				var gotJobNames []string
				for _, sc := range got.PromConfig.ScrapeConfigs {
					gotJobNames = append(gotJobNames, sc.JobName)
				}
				assert.Equal(t, tt.wantJobNames, gotJobNames)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name        string
		fileConfig  Config
		expectedErr error
	}{
		{
			name:        "promCR enabled, no Prometheus config",
			fileConfig:  Config{PromConfig: nil, PrometheusCR: PrometheusCRConfig{Enabled: true}},
			expectedErr: nil,
		},
		{
			name:        "promCR disabled, no Prometheus config",
			fileConfig:  Config{PromConfig: nil},
			expectedErr: fmt.Errorf("at least one scrape config must be defined, or Prometheus CR watching must be enabled"),
		},
		{
			name:        "promCR disabled, Prometheus config present, no scrapeConfigs",
			fileConfig:  Config{PromConfig: &promconfig.Config{}},
			expectedErr: fmt.Errorf("at least one scrape config must be defined, or Prometheus CR watching must be enabled"),
		},
		{
			name: "promCR disabled, Prometheus config present, scrapeConfigs present",
			fileConfig: Config{
				PromConfig: &promconfig.Config{ScrapeConfigs: []*promconfig.ScrapeConfig{{}}},
			},
			expectedErr: nil,
		},
		{
			name: "promCR enabled, Prometheus config present, scrapeConfigs present",
			fileConfig: Config{
				PromConfig:   &promconfig.Config{ScrapeConfigs: []*promconfig.ScrapeConfig{{}}},
				PrometheusCR: PrometheusCRConfig{Enabled: true},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(&tc.fileConfig)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGetAllocationFallbackStrategy(t *testing.T) {
	// Unset: no fallback.
	assert.Equal(t, "", Config{}.GetAllocationFallbackStrategy())

	// Set: returns the configured value.
	strategy := "consistent-hashing"
	assert.Equal(t, strategy, Config{FallbackAllocationStrategy: &strategy}.GetAllocationFallbackStrategy())
}
