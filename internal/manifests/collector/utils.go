// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func getDNSPolicy(otelcol v1alpha1.OpenTelemetryCollector) corev1.DNSPolicy {
	dnsPolicy := corev1.DNSClusterFirst
	if otelcol.Spec.HostNetwork {
		dnsPolicy = corev1.DNSClusterFirstWithHostNet
	}
	return dnsPolicy
}
