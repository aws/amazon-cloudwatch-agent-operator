// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"encoding/json"
	ta "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	promconfig "github.com/prometheus/prometheus/config"
	"time"

	_ "github.com/prometheus/prometheus/discovery/install" // Package install has the side-effect of registering all builtin.

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

type targetAllocator struct {
	Endpoint    string        `yaml:"endpoint"`
	Interval    time.Duration `yaml:"interval"`
	CollectorID string        `yaml:"collector_id"`
	// HTTPSDConfig is a preference that can be set for the collector's target allocator, but the operator doesn't
	// care about what the value is set to. We just need this for validation when unmarshalling the configmap.
	HTTPSDConfig interface{} `yaml:"http_sd_config,omitempty"`
}

type Config struct {
	PromConfig        *promconfig.Config `yaml:"config"`
	TargetAllocConfig *targetAllocator   `yaml:"target_allocator,omitempty"`
}

func ReplaceConfig(instance v1alpha1.AmazonCloudWatchAgent, targetAllocator *v1alpha1.TargetAllocator) (string, error) {
	collectorSpec := instance.Spec
	taEnabled := targetAllocator != nil
	cfgStr, err := collectorSpec.Config.Yaml()
	if err != nil {
		return "", err
	}
	// Check if TargetAllocator is present, if not, return the original config
	if !taEnabled {
		return cfgStr, nil
	}

	config, err := adapters.ConfigFromJSONString(instance.Spec.Config)
	if err != nil {
		return "", err
	}

	promCfgMap, getCfgPromErr := ta.ConfigToPromConfig(cfgStr)
	if getCfgPromErr != nil {
		return "", getCfgPromErr
	}

	validateCfgPromErr := ta.ValidatePromConfig(promCfgMap, taEnabled)
	if validateCfgPromErr != nil {
		return "", validateCfgPromErr
	}

	// To avoid issues caused by Prometheus validation logic, which fails regex validation when it encounters
	// $$ in the prom config, we update the YAML file directly without marshaling and unmarshalling.
	updPromCfgMap, getCfgPromErr := ta.AddTAConfigToPromConfig(promCfgMap, naming.TAService(targetAllocator.Name), targetAllocator.Namespace)
	if getCfgPromErr != nil {
		return "", getCfgPromErr
	}

	// type coercion checks are handled in the AddTAConfigToPromConfig method above
	config["receivers"].(map[interface{}]interface{})["prometheus"] = updPromCfgMap

	out, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
