// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	_ "github.com/prometheus/prometheus/discovery/install" // Package install has the side-effect of registering all builtin.
	"gopkg.in/yaml.v2"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

func ReplaceConfig(instance v1alpha1.AmazonCloudWatchAgent) (string, error) {
	config, err := adapters.ConfigFromString(instance.Spec.Config)
	if err != nil {
		return "", err
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
