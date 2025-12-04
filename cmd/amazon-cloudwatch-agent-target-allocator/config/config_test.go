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
		name    string
		args    args
		want    Config
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "file sd load",
			args: args{
				file: "./testdata/config_test.yaml",
			},
			want: Config{
				AllocationStrategy: &defaulAllocationStrategy,
				LabelSelector: map[string]string{
					"app.kubernetes.io/instance":   "default.test",
					"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
				},
				PrometheusCR: PrometheusCRConfig{
					ScrapeInterval: model.Duration(time.Second * 60),
				},
				HTTPS: HTTPSServerConfig{
					Enabled:         true,
					ListenAddr:      DefaultListenAddr,
					CAFilePath:      "/path/to/ca.pem",
					TLSCertFilePath: "/path/to/cert.pem",
					TLSKeyFilePath:  "/path/to/key.pem",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "no config",
			args: args{
				file: "./testdata/no_config.yaml",
			},
			want:    CreateDefaultConfig(),
			wantErr: assert.NoError,
		},
		{
			name: "service monitor pod monitor selector",
			args: args{
				file: "./testdata/pod_service_selector_test.yaml",
			},
			want: Config{
				AllocationStrategy: &defaulAllocationStrategy,
				LabelSelector: map[string]string{
					"app.kubernetes.io/instance":   "default.test",
					"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
				},
				PrometheusCR: PrometheusCRConfig{
					ScrapeInterval: DefaultCRScrapeInterval,
				},
				HTTPS: HTTPSServerConfig{
					Enabled:         true,
					ListenAddr:      DefaultListenAddr,
					CAFilePath:      DefaultCABundlePath,
					TLSCertFilePath: DefaultTLSCertPath,
					TLSKeyFilePath:  DefaultTLSKeyPath,
				},
				PodMonitorSelector: map[string]string{
					"release": "test",
				},
				ServiceMonitorSelector: map[string]string{
					"release": "test",
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateDefaultConfig()
			err := LoadFromFile(tt.args.file, &got)
			if !tt.wantErr(t, err, fmt.Sprintf("Load(%v)", tt.args.file)) {
				return
			}
			// Compare only the fields we explicitly set, not the entire PromConfig
			// since Prometheus library sets many default values that change between versions
			assert.Equalf(t, tt.want.AllocationStrategy, got.AllocationStrategy, "AllocationStrategy mismatch")
			assert.Equalf(t, tt.want.LabelSelector, got.LabelSelector, "LabelSelector mismatch")
			assert.Equalf(t, tt.want.PrometheusCR, got.PrometheusCR, "PrometheusCR mismatch")
			assert.Equalf(t, tt.want.HTTPS, got.HTTPS, "HTTPS mismatch")
			assert.Equalf(t, tt.want.PodMonitorSelector, got.PodMonitorSelector, "PodMonitorSelector mismatch")
			assert.Equalf(t, tt.want.ServiceMonitorSelector, got.ServiceMonitorSelector, "ServiceMonitorSelector mismatch")

			// Verify PromConfig was loaded (check scrape configs exist)
			if tt.name != "no config" {
				assert.NotNil(t, got.PromConfig, "PromConfig should not be nil")
				assert.Greater(t, len(got.PromConfig.ScrapeConfigs), 0, "ScrapeConfigs should not be empty")
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
