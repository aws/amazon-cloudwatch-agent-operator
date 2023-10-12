// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	corev1 "k8s.io/api/core/v1"
)

var CloudwatchAgentPorts = []corev1.ServicePort{
	{
		Name: "otlp-grpc",
		Port: 4317,
	},
	{
		Name: "otlp-http",
		Port: 4318,
	},
	{
		Name: "aws-proxy",
		Port: 2000,
	},
}
