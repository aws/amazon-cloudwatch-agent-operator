// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"net"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/mitchellh/mapstructure"
)

type ComponentType int

const (
	ComponentTypeReceiver ComponentType = iota
	ComponentTypeExporter
)

func (c ComponentType) String() string {
	return [...]string{"receiver", "exporter"}[c]
}

// ConfigToMetricsPort gets the port number for the metrics endpoint from the collector config if it has been set.
func ConfigToMetricsPort(logger logr.Logger, config map[interface{}]interface{}) (int32, error) {
	// we don't need to unmarshal the whole config, just follow the keys down to
	// the metrics address.
	type metricsCfg struct {
		Address string
	}
	type telemetryCfg struct {
		Metrics metricsCfg
	}
	type serviceCfg struct {
		Telemetry telemetryCfg
	}
	type cfg struct {
		Service serviceCfg
	}
	var cOut cfg
	err := mapstructure.Decode(config, &cOut)
	if err != nil {
		return 0, err
	}

	_, port, netErr := net.SplitHostPort(cOut.Service.Telemetry.Metrics.Address)
	if netErr != nil && strings.Contains(netErr.Error(), "missing port in address") {
		return 8888, nil
	} else if netErr != nil {
		return 0, netErr
	}
	i64, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(i64), nil
}
