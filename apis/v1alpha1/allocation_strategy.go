// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type (
	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategy represent which strategy to distribute target to each collector
	// +kubebuilder:validation:Enum=least-weighted;consistent-hashing
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategy string
)

const (
	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategyLeastWeighted targets will be distributed to collector with fewer targets currently assigned.
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategyLeastWeighted AmazonCloudWatchAgentTargetAllocatorAllocationStrategy = "least-weighted"

	// AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing targets will be consistently added to collectors, which allows a high-availability setup.
	AmazonCloudWatchAgentTargetAllocatorAllocationStrategyConsistentHashing AmazonCloudWatchAgentTargetAllocatorAllocationStrategy = "consistent-hashing"
)
