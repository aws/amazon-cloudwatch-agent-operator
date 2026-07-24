// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/diff"
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


// jobFilter keeps only targets whose job name matches keep.
type jobFilter struct{ keep string }

func (f jobFilter) Apply(in map[string]*target.Item) map[string]*target.Item {
	out := map[string]*target.Item{}
	for k, v := range in {
		if v.JobName == f.keep {
			out[k] = v
		}
	}
	return out
}

// TestPerNodeSetFilter verifies a configured filter is applied to incoming
// targets before allocation.
func TestPerNodeSetFilter(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetFilter(jobFilter{keep: "keep"})
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	keep := nodeTarget("keep", "10.0.0.1:8080", "node-a")
	drop := nodeTarget("drop", "10.0.0.2:8080", "node-a")
	c.SetTargets(map[string]*target.Item{keep.Hash(): keep, drop.Hash(): drop})

	require.Len(t, c.TargetItems(), 1, "filtered-out target must not be tracked")
	assigned := c.GetTargetsForCollectorAndJob("collector-a", "keep")
	require.Len(t, assigned, 1)
	assert.Equal(t, keep.Hash(), assigned[0].Hash())
}

// TestPerNodeUnsupportedFallbackDisabled verifies that requesting an unsupported
// fallback strategy leaves the fallback disabled, so node-less targets stay
// unassigned.
func TestPerNodeUnsupportedFallbackDisabled(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetFallbackStrategy("bogus-strategy")
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	noNode := nodeTarget("job1", "10.0.0.9:8080", "")
	c.SetTargets(map[string]*target.Item{noNode.Hash(): noNode})

	require.Len(t, c.TargetItems(), 1)
	for _, ti := range c.TargetItems() {
		assert.Equal(t, "", ti.CollectorName, "node-less target must stay unassigned when fallback is unsupported")
	}
}

// TestPerNodeFallbackSeedsExistingCollectors verifies enabling the fallback
// after collectors already exist still places node-less targets (the ring is
// seeded with the known collectors).
func TestPerNodeFallbackSeedsExistingCollectors(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})
	// Enable fallback AFTER collectors are known, exercising the seed loop.
	c.SetFallbackStrategy(consistentHashingStrategyName)

	noNode := nodeTarget("job1", "10.0.0.9:8080", "")
	c.SetTargets(map[string]*target.Item{noNode.Hash(): noNode})

	for _, ti := range c.TargetItems() {
		assert.Equal(t, "collector-a", ti.CollectorName, "node-less target must be placed by the seeded fallback ring")
	}
}

// TestPerNodeTargetRemoval verifies removed targets are dropped from the pool
// and decremented from their owning collector.
func TestPerNodeTargetRemoval(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	t1 := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	t2 := nodeTarget("job1", "10.0.0.2:8080", "node-a")
	c.SetTargets(map[string]*target.Item{t1.Hash(): t1, t2.Hash(): t2})
	require.Len(t, c.GetTargetsForCollectorAndJob("collector-a", "job1"), 2)

	// Remove t2.
	c.SetTargets(map[string]*target.Item{t1.Hash(): t1})
	assert.Len(t, c.TargetItems(), 1)
	assert.Len(t, c.GetTargetsForCollectorAndJob("collector-a", "job1"), 1)
	assert.Equal(t, 1, c.Collectors()["collector-a"].NumTargets)
}

// TestPerNodeCollectorRemovalReassigns verifies that when a collector is
// removed, its node's targets are re-evaluated (and fall back / unassign), and
// that adding another collector re-allocates already-assigned targets
// (exercising the reassignment decrement path).
func TestPerNodeCollectorRemovalReassigns(t *testing.T) {
	a, err := New(perNodeStrategyName, logger, WithFallbackStrategy(consistentHashingStrategyName))
	require.NoError(t, err)
	c := a.(*perNodeAllocator)

	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})
	ta := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	tb := nodeTarget("job1", "10.0.0.3:8080", "node-b")
	c.SetTargets(map[string]*target.Item{ta.Hash(): ta, tb.Hash(): tb})
	require.Len(t, c.GetTargetsForCollectorAndJob("collector-a", "job1"), 1)

	// Remove collector-a: its node-a target must leave collector-a. With the
	// fallback enabled it lands on a remaining collector rather than vanishing.
	c.SetCollectors(map[string]*Collector{
		"collector-b": NewCollector("collector-b", "node-b"),
	})
	assert.Empty(t, c.GetTargetsForCollectorAndJob("collector-a", "job1"))
	// Every target is still tracked and assigned to a live collector.
	for _, ti := range c.TargetItems() {
		_, live := c.Collectors()[ti.CollectorName]
		assert.True(t, live, "target must be owned by a live collector after removal")
	}
}

// TestPerNodeSetTargetsBeforeCollectors verifies targets discovered before any
// collector exists are tracked (unassigned) and later removable in that state.
func TestPerNodeSetTargetsBeforeCollectors(t *testing.T) {
	c := newPerNodeTestAllocator()

	t1 := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	t2 := nodeTarget("job1", "10.0.0.2:8080", "node-a")
	c.SetTargets(map[string]*target.Item{t1.Hash(): t1, t2.Hash(): t2})
	require.Len(t, c.TargetItems(), 2)
	for _, ti := range c.TargetItems() {
		assert.Equal(t, "", ti.CollectorName, "no collectors yet, targets must be unassigned")
	}

	// Removal while still collector-less.
	c.SetTargets(map[string]*target.Item{t1.Hash(): t1})
	assert.Len(t, c.TargetItems(), 1)
}

// TestPerNodeGetTargetsUnknownJob verifies querying a known collector for a job
// it does not own returns an empty slice, not nil-panic.
func TestPerNodeGetTargetsUnknownJob(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})
	tg := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	c.SetTargets(map[string]*target.Item{tg.Hash(): tg})

	assert.Empty(t, c.GetTargetsForCollectorAndJob("collector-a", "no-such-job"))
	assert.Empty(t, c.GetTargetsForCollectorAndJob("no-such-collector", "job1"))
}

// TestPerNodeFallbackDuringCollectorChange verifies a node-less target already
// placed by the fallback is re-evaluated through the fallback again when the
// collector set changes (exercises the fallback arm of the collector-change
// re-allocation loop).
func TestPerNodeFallbackDuringCollectorChange(t *testing.T) {
	a, err := New(perNodeStrategyName, logger, WithFallbackStrategy(consistentHashingStrategyName))
	require.NoError(t, err)
	c := a.(*perNodeAllocator)

	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})
	noNode := nodeTarget("job1", "10.0.0.9:8080", "")
	c.SetTargets(map[string]*target.Item{noNode.Hash(): noNode})
	require.NotEqual(t, "", c.TargetItems()[noNode.Hash()].CollectorName)

	// Add another collector: the node-less target is re-allocated via the
	// fallback during handleCollectors.
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})
	placed := c.TargetItems()[noNode.Hash()].CollectorName
	assert.NotEqual(t, "", placed, "node-less target must remain fallback-placed after a collector change")
	_, live := c.Collectors()[placed]
	assert.True(t, live)
}

// TestPerNodeSetCollectorsEmpty verifies passing an empty collector set is a
// safe no-op (early return) and does not panic or assign anything.
func TestPerNodeSetCollectorsEmpty(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{})
	assert.Empty(t, c.Collectors())
}

// TestPerNodeUnplacedDuringCollectorChange verifies that, with no fallback, a
// target whose owning collector is removed becomes unassigned during the
// collector-change re-allocation (exercises the unplaced arm of that loop).
func TestPerNodeUnplacedDuringCollectorChange(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
		"collector-b": NewCollector("collector-b", "node-b"),
	})
	tb := nodeTarget("job1", "10.0.0.3:8080", "node-b")
	c.SetTargets(map[string]*target.Item{tb.Hash(): tb})
	require.Len(t, c.GetTargetsForCollectorAndJob("collector-b", "job1"), 1)

	// Remove collector-b; its node-b target has nowhere to go (no fallback).
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})
	assert.Empty(t, c.GetTargetsForCollectorAndJob("collector-b", "job1"))
	assert.Equal(t, "", c.TargetItems()[tb.Hash()].CollectorName, "target must be unassigned after its collector is removed")
}

// TestPerNodeHandleTargetsSkipsAlreadyTracked is a white-box test for the
// defensive guard in handleTargets that skips an "addition" whose target is
// already tracked, so it is never counted or placed twice. This state is not
// reachable through SetTargets (which diffs against the current pool), so the
// guard is exercised by driving handleTargets directly.
func TestPerNodeHandleTargetsSkipsAlreadyTracked(t *testing.T) {
	c := newPerNodeTestAllocator()
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	tg := nodeTarget("job1", "10.0.0.1:8080", "node-a")
	// Seed the target as already tracked (and unassigned).
	c.targetItems[tg.Hash()] = tg

	// A diff that presents the same target as an addition must be skipped.
	changes := diff.Maps(map[string]*target.Item{}, map[string]*target.Item{tg.Hash(): tg})
	c.handleTargets(changes)

	assert.Len(t, c.TargetItems(), 1, "already-tracked target must not be added twice")
	assert.Equal(t, 0, c.Collectors()["collector-a"].NumTargets, "guarded target must not be (re)assigned")
}


// TestPerNodeWarnsOnceWhenNoFallback verifies that a per-node allocator with no
// fallback logs the "no fallback strategy configured" warning exactly once (not
// per target) and leaves node-less targets unassigned.
func TestPerNodeWarnsOnceWhenNoFallback(t *testing.T) {
	var msgs []string
	capLogger := funcr.New(func(prefix, args string) { msgs = append(msgs, args) }, funcr.Options{})

	c := newPerNodeAllocator(capLogger).(*perNodeAllocator)
	c.SetCollectors(map[string]*Collector{
		"collector-a": NewCollector("collector-a", "node-a"),
	})

	// Two node-less targets: with no fallback both stay unassigned, and the
	// warning must fire exactly once.
	n1 := nodeTarget("job1", "10.0.0.9:8080", "")
	n2 := nodeTarget("job1", "10.0.0.10:8080", "")
	c.SetTargets(map[string]*target.Item{n1.Hash(): n1, n2.Hash(): n2})

	for _, ti := range c.TargetItems() {
		if ti.Hash() == n1.Hash() || ti.Hash() == n2.Hash() {
			assert.Equal(t, "", ti.CollectorName, "node-less target must be unassigned without a fallback")
		}
	}

	warnings := 0
	for _, m := range msgs {
		if strings.Contains(m, "no fallback strategy configured") {
			warnings++
		}
	}
	assert.Equal(t, 1, warnings, "no-fallback warning must be logged exactly once")
}


// TestPerNodeTwoCollectorsSameNodeTieBreak verifies that when two collectors
// report the same node (e.g. a transient maxSurge DaemonSet rollout), the node
// is owned deterministically by the smaller pod name, not by map iteration
// order, so target placement doesn't flap.
func TestPerNodeTwoCollectorsSameNodeTieBreak(t *testing.T) {
	// Run several times: SetCollectors iterates a map (random order), so a flaky
	// last-write-wins would eventually pick the larger name.
	for i := 0; i < 50; i++ {
		c := newPerNodeTestAllocator()
		c.SetCollectors(map[string]*Collector{
			"collector-b": NewCollector("collector-b", "node-a"),
			"collector-a": NewCollector("collector-a", "node-a"),
		})
		owner := c.collectorByNode["node-a"]
		require.NotNil(t, owner)
		assert.Equal(t, "collector-a", owner.Name, "smaller pod name must deterministically own the shared node")
	}
}
