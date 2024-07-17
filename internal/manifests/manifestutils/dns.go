// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifestutils

import (
	corev1 "k8s.io/api/core/v1"
)

// GetDNSPolicy Get the Pod DNS Policy depending on whether we're using a host network.
func GetDNSPolicy(hostNetwork bool) corev1.DNSPolicy {
	dnsPolicy := corev1.DNSClusterFirst
	if hostNetwork {
		dnsPolicy = corev1.DNSClusterFirstWithHostNet
	}
	return dnsPolicy
}
