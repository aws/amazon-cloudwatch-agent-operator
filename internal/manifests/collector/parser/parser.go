// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

type ComponentPortParser interface {
	// Ports returns the service ports parsed based on the exporter's configuration
	Ports() ([]corev1.ServicePort, error)

	// ParserName returns the name of this parser
	ParserName() string
}

// Builder specifies the signature required for parser builders.
type Builder func(logr.Logger, string, map[interface{}]interface{}) ComponentPortParser
