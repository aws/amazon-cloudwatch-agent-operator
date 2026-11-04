// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type (
	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategy represent which strategy to distribute target to each collector
	// +kubebuilder:validation:Enum=consistent-hashing;per-node
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategy string
)

const (
	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing targets will be consistently added to collectors, which allows a high-availability setup.
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing AmazonCloudWatchAgentTargetAllocatorAllocationStrategy = "consistent-hashing"

	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategyPerNode targets will be allocated to the collector running on the same node as the target.
	// Targets without a resolvable node fall back to the configured fallback strategy (consistent-hashing).
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategyPerNode AmazonCloudWatchAgentTargetAllocatorAllocationStrategy = "per-node"
)
