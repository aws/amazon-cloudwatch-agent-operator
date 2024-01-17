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
