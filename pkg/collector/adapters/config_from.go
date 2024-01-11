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
	Metrics *metric `json:"metrics,omitempty"`
	Logs    *log    `json:"logs,omitempty"`
	Traces  *trace  `json:"traces,omitempty"`
}

type metric struct {
	MetricsCollected *metricCollected `json:"metrics_collected,omitempty"`
}

type log struct {
	LogMetricsCollected *logMetricCollected `json:"metrics_collected,omitempty"`
}

type trace struct {
	TracesCollected *traceCollected `json:"traces_collected,omitempty"`
}

type metricCollected struct {
	StatsD   *statsD   `json:"statsd,omitempty"`
	CollectD *collectD `json:"collectd,omitempty"`
}

type logMetricCollected struct {
	EMF *emf `json:"emf,omitempty"`
}

type traceCollected struct {
	XRay *xray `json:"xray,omitempty"`
	OTLP *otlp `json:"otlp,omitempty"`
}

type statsD struct {
	ServiceAddress string `json:"service_address,omitempty"`
}

type collectD struct {
	ServiceAddress string `json:"service_address,omitempty"`
}

type emf struct {
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

func ConfigStructFromJSONString(configStr string) (*CwaConfig, error) {
	var config *CwaConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, ErrInvalidJSON
	}

	return config, nil
}
