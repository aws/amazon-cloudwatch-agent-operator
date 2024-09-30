// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser"

	"github.com/go-logr/logr"
	"github.com/mitchellh/mapstructure"

	receiverParser "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/parser/receiver"
)

type ComponentType int

const (
	ComponentTypeReceiver ComponentType = iota
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

func GetServicePortsFromCWAgentOtelConfig(logger logr.Logger, config map[interface{}]interface{}) ([]corev1.ServicePort, error) {
	ports, err := ConfigToComponentPorts(logger, ComponentTypeReceiver, config)
	if err != nil {
		logger.Error(err, "there was a problem while getting the ports from the receivers")
		return nil, err
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})

	return ports, nil
}

// ConfigToComponentPorts converts the incoming configuration object into a set of service ports required by the exporters.
func ConfigToComponentPorts(logger logr.Logger, cType ComponentType, config map[interface{}]interface{}) ([]corev1.ServicePort, error) {
	// now, we gather which ports we might need to open
	// for that, we get all the exporters and check their `endpoint` properties,
	// extracting the port from it. The port name has to be a "DNS_LABEL", so, we try to make it follow the pattern:
	// examples:
	// ```yaml
	// components:
	//   componentexample:
	//     endpoint: 0.0.0.0:12345
	//   componentexample/settings:
	//     endpoint: 0.0.0.0:12346
	// in this case, we have 2 ports, named: "componentexample" and "componentexample-settings"
	componentsProperty, ok := config[fmt.Sprintf("%ss", cType.String())]
	if !ok {
		return nil, fmt.Errorf("no %ss available as part of the configuration", cType)
	}

	components, ok := componentsProperty.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("%ss doesn't contain valid components", cType.String())
	}

	compEnabled := getEnabledComponents(config, cType)

	if compEnabled == nil {
		return nil, fmt.Errorf("no enabled %ss available as part of the configuration", cType)
	}

	ports := []corev1.ServicePort{}
	for key, val := range components {
		// This check will pass only the enabled components,
		// then only the related ports will be opened.
		if !compEnabled[key] {
			continue
		}
		extractedComponent, ok := val.(map[interface{}]interface{})
		if !ok {
			logger.V(2).Info("component doesn't seem to be a map of properties", cType.String(), key)
			extractedComponent = map[interface{}]interface{}{}
		}

		cmptName := key.(string)
		var cmptParser parser.ComponentPortParser
		var err error
		cmptParser, err = receiverParser.For(logger, cmptName, extractedComponent)
		if err != nil {
			logger.V(2).Info("no parser found for '%s'", cmptName)
			continue
		}

		exprtPorts, err := cmptParser.Ports()
		if err != nil {
			logger.Error(err, "parser for '%s' has returned an error: %w", cmptName, err)
			continue
		}

		if len(exprtPorts) > 0 {
			ports = append(ports, exprtPorts...)
		}
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})

	return ports, nil
}
