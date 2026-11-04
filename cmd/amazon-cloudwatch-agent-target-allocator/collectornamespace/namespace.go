// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package collectornamespace resolves the namespace the agent (collector) pods
// run in. The Target Allocator uses it both to watch the agent pods and to scope
// its Prometheus CR discovery, so both must agree.
package collectornamespace

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
)

// EnvVar is the environment variable the operator sets on the Target Allocator
// container from the pod's metadata.namespace.
const EnvVar = "OTELCOL_NAMESPACE"

// Default is used when neither EnvVar nor the service account namespace file
// provide a value.
const Default = "amazon-cloudwatch"

const defaultServiceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// serviceAccountNamespacePath is a variable so tests can redirect the read.
var serviceAccountNamespacePath = defaultServiceAccountNamespacePath

// Resolve returns, in order: EnvVar (trimmed), the namespace of the pod's
// service account (trimmed), or Default. It never returns "", which client-go
// would treat as all namespaces.
func Resolve(logger logr.Logger) string {
	if ns := strings.TrimSpace(os.Getenv(EnvVar)); ns != "" {
		return ns
	}
	namespace := Default
	if data, err := os.ReadFile(serviceAccountNamespacePath); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			namespace = ns
		}
	}
	logger.Info(EnvVar+" not set, resolved namespace", "namespace", namespace)
	return namespace
}
