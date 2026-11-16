// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	serviceMonitorSelectorKey = "service_monitor_selector"
	podMonitorSelectorKey     = "pod_monitor_selector"
)

// labelSelectorFieldNames maps the field names accepted in a selector object to
// the camelCase names metav1.LabelSelector uses in its json tags. The lowercase
// forms are what gopkg.in/yaml.v2 emits when it marshals a metav1.LabelSelector
// (it has no yaml tags), which is how the operator writes this config. When both
// forms are present the camelCase one wins, matching the upstream
// OpenTelemetry Target Allocator.
var labelSelectorFieldNames = map[string]string{
	"matchlabels":      "matchLabels",
	"matchexpressions": "matchExpressions",
	"matchLabels":      "matchLabels",
	"matchExpressions": "matchExpressions",
}

// UnmarshalYAML decodes the Target Allocator config. The ServiceMonitor and
// PodMonitor selectors are decoded separately so that they accept both the
// label selector object form (matchLabels and matchExpressions, in camelCase or
// lowercase) and the legacy flat {key: value} map, which is read as matchLabels.
// Every other field is decoded strictly, as before.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := map[string]interface{}{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	selectors := map[string]**metav1.LabelSelector{
		serviceMonitorSelectorKey: &c.ServiceMonitorSelector,
		podMonitorSelectorKey:     &c.PodMonitorSelector,
	}
	for key, target := range selectors {
		value, ok := raw[key]
		if !ok {
			continue
		}
		delete(raw, key)
		selector, err := decodeLabelSelector(value)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*target = selector
	}

	rest, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	// plain has the same fields as Config but not its UnmarshalYAML method.
	type plain Config
	return yaml.UnmarshalStrict(rest, (*plain)(c))
}

// decodeLabelSelector converts a decoded YAML value into a label selector. A null
// value yields a nil selector. A map whose keys are all label selector fields is
// read as a label selector object; a map with none of them is read as the legacy
// flat form and becomes matchLabels. A map that mixes the two is rejected.
func decodeLabelSelector(value interface{}) (*metav1.LabelSelector, error) {
	if value == nil {
		return nil, nil
	}
	fields, ok := value.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a map, got %T", value)
	}

	var selectorKeys, labelKeys []string
	for k := range fields {
		key := fmt.Sprint(k)
		if _, isField := labelSelectorFieldNames[key]; isField {
			selectorKeys = append(selectorKeys, key)
		} else {
			labelKeys = append(labelKeys, key)
		}
	}
	if len(selectorKeys) > 0 && len(labelKeys) > 0 {
		sort.Strings(selectorKeys)
		sort.Strings(labelKeys)
		return nil, fmt.Errorf("cannot mix label selector fields (%s) with flat label keys (%s)",
			strings.Join(selectorKeys, ", "), strings.Join(labelKeys, ", "))
	}

	if len(selectorKeys) == 0 {
		b, err := yaml.Marshal(fields)
		if err != nil {
			return nil, err
		}
		matchLabels := map[string]string{}
		if err := yaml.UnmarshalStrict(b, &matchLabels); err != nil {
			return nil, err
		}
		if len(matchLabels) == 0 {
			return &metav1.LabelSelector{}, nil
		}
		return &metav1.LabelSelector{MatchLabels: matchLabels}, nil
	}

	normalized := map[string]interface{}{}
	// Lowercase first so that a camelCase field overwrites it.
	for _, key := range []string{"matchlabels", "matchexpressions", "matchLabels", "matchExpressions"} {
		if v, ok := fields[key]; ok {
			normalized[labelSelectorFieldNames[key]] = v
		}
	}
	b, err := yaml.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	selector := &metav1.LabelSelector{}
	if err := sigsyaml.UnmarshalStrict(b, selector); err != nil {
		return nil, err
	}
	return selector, nil
}

// MonitorSelector converts a ServiceMonitor or PodMonitor selector into a
// labels.Selector. A nil or empty selector selects every monitor. An invalid
// selector, such as an unknown operator or In with no values, is an error rather
// than a selector that silently matches everything.
func MonitorSelector(s *metav1.LabelSelector) (labels.Selector, error) {
	if s == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(s)
}
