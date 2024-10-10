// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package adapters is for data conversion.
package adapters

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v2"
)

var (
	// ErrInvalidYAML represents an error in the format of the configuration file.
	ErrInvalidYAML = errors.New("couldn't parse the yaml configuration")
	ErrInvalidJSON = errors.New("couldn't parse cloudwatch agent json configuration")
)

// ConfigFromString extracts a configuration map from the given string.
// If the given string isn't a valid YAML, ErrInvalidYAML is returned.
func ConfigFromString(configStr string) (map[interface{}]interface{}, error) {
	config := make(map[interface{}]interface{})
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, ErrInvalidYAML
	}

	return config, nil
}

func ConfigFromJSONString(configStr string) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, ErrInvalidJSON
	}

	return config, nil
}

type CwaConfig struct {
	Metrics *Metrics `json:"metrics,omitempty"`
	Logs    *Logs    `json:"logs,omitempty"`
	Traces  *Traces  `json:"traces,omitempty"`
}

type Metrics struct {
	MetricsCollected *MetricsCollected `json:"metrics_collected,omitempty"`
}

type Logs struct {
	LogMetricsCollected *LogMetricsCollected `json:"metrics_collected,omitempty"`
}

type Traces struct {
	TracesCollected *TracesCollected `json:"traces_collected,omitempty"`
}

type MetricsCollected struct {
	StatsD   *statsD   `json:"statsd,omitempty"`
	CollectD *collectD `json:"collectd,omitempty"`
	JMX      *jmx      `json:"jmx,omitempty"`
}

type LogMetricsCollected struct {
	EMF                *emf        `json:"emf,omitempty"`
	ApplicationSignals *AppSignals `json:"application_signals,omitempty"`
	AppSignals         *AppSignals `json:"app_signals,omitempty"`
	Kubernetes         *kubernetes `json:"kubernetes,omitempty"`
}

type TracesCollected struct {
	XRay *xray `json:"xray,omitempty"`
	OTLP *otlp `json:"otlp,omitempty"`
}

type statsD struct {
	ServiceAddress string `json:"service_address,omitempty"`
}

type collectD struct {
	ServiceAddress string `json:"service_address,omitempty"`
}

type AppSignals struct {
	TLS *TLS `json:"tls,omitempty"`
}

type emf struct {
}

type jmx struct{}

type kubernetes struct {
	EnhancedContainerInsights bool `json:"enhanced_container_insights,omitempty"`
	AcceleratedComputeMetrics bool `json:"accelerated_compute_metrics,omitempty"`
}

type xray struct {
	BindAddress string    `json:"bind_address,omitempty"`
	TCPProxy    *tcpProxy `json:"tcp_proxy,omitempty"`
}

type tcpProxy struct {
	BindAddress string `json:"bind_address,omitempty"`
}

type otlp struct {
	GRPCEndpoint string `json:"grpc_endpoint,omitempty"`
	HTTPEndpoint string `json:"http_endpoint,omitempty"`
}

type TLS struct {
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
}

func ConfigStructFromJSONString(configStr string) (*CwaConfig, error) {
	var config *CwaConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *CwaConfig) GetApplicationSignalsConfig() *AppSignals {
	if c.Logs == nil {
		return nil
	}
	if c.Logs.LogMetricsCollected == nil {
		return nil
	}
	if c.Logs.LogMetricsCollected.ApplicationSignals != nil {
		return c.Logs.LogMetricsCollected.ApplicationSignals
	}
	if c.Logs.LogMetricsCollected.AppSignals != nil {
		return c.Logs.LogMetricsCollected.AppSignals
	}
	return nil
}
