// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/target"
)

// nodeTarget builds a target carrying the pod node-name discovery label.
func nodeTarget(job, url, node string) *target.Item {
	lbls := model.LabelSet{}
	if node != "" {
		lbls[model.LabelName("__meta_kubernetes_pod_node_name")] = model.LabelValue(node)
	}
	return target.NewItem(job, url, lbls, "")
}

func newPerNodeTestAllocator() *perNodeAllocator {
	return newPerNodeAllocator(logger).(*perNodeAllocator)
}

// TestPerNodeRegistered ensures the strategy is wired into the registry.
func TestPerNodeRegistered(t *testing.T) {
	a, err := New(perNodeStrategyName, logger)
	require.NoError(t, err)
	require.NotNil(t, a)
}

// TestPerNodeAssignsToMatchingNode verifies each target lands on the collector
// running on the target's node, and only that collector.
func TestPerNodeAssignsToMatchingNode(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})

	targets := map[string]*target.Item{}
	for _, tg := range []*target.Item{
		nodeTarget("job1", "10.0.0.1:8080", "node-a"),
		nodeTarget("job1", "10.0.0.2:8080", "node-a"),
		nodeTarget("job1", "10.0.0.3:8080", "node-b"),
	} {
		targets[tg.Hash()] = tg
	}
	c.SetTargets(targets)

	assert.Len(t, c.TargetItems(), 3)

	aTargets := c.GetTargetsForCollectorAndJob("collector-a", "job1")
	bTargets := c.GetTargetsForCollectorAndJob("collector-b", "job1")
	assert.Len(t, aTargets, 2, "node-a collector should own both node-a targets")
	assert.Len(t, bTargets, 1, "node-b collector should own the single node-b target")

	for _, ti := range aTargets {
		assert.Equal(t, "collector-a", ti.CollectorName)
		assert.Equal(t, "node-a", ti.GetNodeName())
	}
	for _, ti := range bTargets {
		assert.Equal(t, "collector-b", ti.CollectorName)
		assert.Equal(t, "node-b", ti.GetNodeName())
	}
}

// TestPerNodeLeavesNodelessTargetUnassigned verifies a target without a node
// label is retained but assigned to no collector.
func TestPerNodeLeavesNodelessTargetUnassigned(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	onNode := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	noNode := nodeTarget("job1", "10.0.0.9:8080", "") // external/non-pod endpoint
	c.SetTargets(map[string]*target.Item{
		onNode.Hash(): onNode,
		noNode.Hash(): noNode,
	})

	// Both are tracked.
	assert.Len(t, c.TargetItems(), 2)
	// Only the node-matched target is assigned to the collector.
	assigned := c.GetTargetsForCollectorAndJob("collector-a", "job1")
	assert.Len(t, assigned, 1)
	assert.Equal(t, onNode.Hash(), assigned[0].Hash())

	// The node-less target carries no collector.
	for _, ti := range c.TargetItems() {
		if ti.Hash() == noNode.Hash() {
			assert.Equal(t, "", ti.CollectorName, "node-less target must stay unassigned")
		}
	}
}

// TestPerNodeFallbackAssignsNodelessTarget verifies that, with a consistent-hashing
// fallback configured, a target without a node is still placed on some collector
// rather than left unassigned.
func TestPerNodeFallbackAssignsNodelessTarget(t *testing.T) {
	a, err := New(perNodeStrategyName, logger, WithFallbackStrategy(consistentHashingStrategyName))
	require.NoError(t, err)
	c := a.(*perNodeAllocator)

	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})

	onNode := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	noNode := nodeTarget("job1", "10.0.0.9:8080", "") // external/non-pod endpoint
	c.SetTargets(map[string]*target.Item{
		onNode.Hash(): onNode,
		noNode.Hash(): noNode,
	})

	// node-matched target lands on its node's collector.
	assert.Equal(t, "collector-a", onNode.CollectorName)

	// node-less target is assigned to *some* collector via the fallback (not "").
	var found *target.Item
	for _, ti := range c.TargetItems() {
		if ti.Hash() == noNode.Hash() {
			found = ti
		}
	}
	require.NotNil(t, found)
	assert.NotEqual(t, "", found.CollectorName, "node-less target must be placed by the consistent-hashing fallback")
	_, isRealCollector := c.Collectors()[found.CollectorName]
	assert.True(t, isRealCollector, "fallback must assign to a known collector")
}

// target (its node had no collector) gets placed once a matching collector joins.
func TestPerNodeReallocatesWhenCollectorAppears(t *testing.T) {
	c := newPerNodeTestAllocator()
	// Only node-a has a collector initially.
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	onB := nodeTarget("job1", "10.0.0.3:8080", "node-b")
	c.SetTargets(map[string]*target.Item{onB.Hash(): onB})

	// node-b target cannot be placed yet.
	assert.Empty(t, c.GetTargetsForCollectorAndJob("collector-b", "job1"))

	// node-b collector joins; target should now be assigned to it.
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})

	bTargets := c.GetTargetsForCollectorAndJob("collector-b", "job1")
	require.Len(t, bTargets, 1)
	assert.Equal(t, "collector-b", bTargets[0].CollectorName)
}
