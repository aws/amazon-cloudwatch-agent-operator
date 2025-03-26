// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import "github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"

// AnnotationConfig details the resources that have enabled
// auto-annotation for each instrumentation type.
type MonitorConfig struct {
	MonitorAllServices bool                    `json:"monitorAllServices"`
	Languages          instrumentation.TypeSet `json:"languages"`
	RestartPods        bool                    `json:"restartPods"`
	Exclude            struct {
		Namespaces []string `json:"namespaces"`
		Services   []string `json:"services"`
	} `json:"exclude"`
	CustomSelector AnnotationConfig `json:"customSelector"`
}
