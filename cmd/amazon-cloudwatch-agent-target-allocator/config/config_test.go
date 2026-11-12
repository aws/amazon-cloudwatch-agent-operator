// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoad(t *testing.T) {
	var defaulAllocationStrategy = DefaultAllocationStrategy
	type args struct {
		file string
	}
	tests := []struct {
		name          string
		args          args
		wantErr       assert.ErrorAssertionFunc
		wantHTTPS     HTTPSServerConfig
		wantLabels    map[string]string
		wantPromCR    PrometheusCRConfig
		wantAlloc     *string
		wantPodMonSel *metav1.LabelSelector
		wantSvcMonSel *metav1.LabelSelector
		wantJobNames  []string
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
			wantErr:    assert.NoError,
			wantHTTPS:  CreateDefaultConfig().HTTPS,
			wantLabels: nil,
			wantPromCR: CreateDefaultConfig().PrometheusCR,
			wantAlloc:  CreateDefaultConfig().AllocationStrategy,
		},
		{
			name: "legacy flat map service monitor pod monitor selector",
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
			wantPodMonSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"release": "test"},
			},
			wantSvcMonSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"release": "test"},
			},
			wantJobNames: []string{"prometheus"},
		},
		{
			name: "label selector object service monitor pod monitor selector",
			args: args{
				file: "./testdata/pod_service_label_selector_test.yaml",
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
			wantPodMonSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"release": "test"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web", "api"}},
				},
			},
			wantSvcMonSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"release": "test"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "team", Operator: metav1.LabelSelectorOpExists},
				},
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
			name: "promCR enabled, invalid service monitor selector",
			fileConfig: Config{
				PrometheusCR: PrometheusCRConfig{Enabled: true},
				ServiceMonitorSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpIn},
				}},
			},
			expectedErr: fmt.Errorf("invalid service_monitor_selector: %w",
				errors.New("values: Invalid value: null: for 'in', 'notin' operators, values set can't be empty")),
		},
		{
			name: "promCR enabled, invalid pod monitor selector",
			fileConfig: Config{
				PrometheusCR: PrometheusCRConfig{Enabled: true},
				PodMonitorSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: "Equals", Values: []string{"prod"}},
				}},
			},
			expectedErr: fmt.Errorf("invalid pod_monitor_selector: %w",
				errors.New(`"Equals" is not a valid label selector operator`)),
		},
		{
			name: "promCR enabled, valid selectors",
			fileConfig: Config{
				PrometheusCR:           PrometheusCRConfig{Enabled: true},
				ServiceMonitorSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"release": "test"}},
				PodMonitorSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"dev"}},
				}},
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
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestGetAllocationFallbackStrategy(t *testing.T) {
	// Unset with the default (consistent-hashing) strategy: no fallback.
	assert.Equal(t, "", Config{}.GetAllocationFallbackStrategy())

	// Set: returns the configured value.
	strategy := "consistent-hashing"
	assert.Equal(t, strategy, Config{FallbackAllocationStrategy: &strategy}.GetAllocationFallbackStrategy())

	// Unset with the per-node strategy: defaults to consistent-hashing so
	// node-less targets are still scraped.
	perNode := PerNodeAllocationStrategy
	assert.Equal(t, DefaultPerNodeFallbackStrategy,
		Config{AllocationStrategy: &perNode}.GetAllocationFallbackStrategy())

	// Explicitly empty with per-node: an opt-out, fallback stays disabled.
	none := ""
	assert.Equal(t, "", Config{AllocationStrategy: &perNode, FallbackAllocationStrategy: &none}.GetAllocationFallbackStrategy())
}
