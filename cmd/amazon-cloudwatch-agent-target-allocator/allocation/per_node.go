// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"sort"
	"strings"
	"sync"

	"github.com/buraksezer/consistent"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/diff"
	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/target"
)

var _ Allocator = &perNodeAllocator{}

const perNodeStrategyName = "per-node"

// placement is the outcome of allocating a single target, used for per-cycle
// log summaries describing what the per-node strategy did.
type placement int

const (
	placedByNode     placement = iota // assigned to the collector on the target's node
	placedByFallback                  // assigned via the consistent-hashing fallback
	unplaced                          // left unassigned (no node match, no fallback)
)

// perNodeAllocator assigns each target to the collector running on the same
// Kubernetes node as the target. It mirrors the node-lookup logic of the
// upstream OpenTelemetry target allocator's per-node strategy
// (cmd/otel-allocator/internal/allocation/per_node.go), adapted to this fork's
// fused Allocator model (see consistent_hashing.go where the allocator and the
// placement strategy are a single type).
//
// Placement: a target's node comes from target.Item.GetNodeName() (the
// __meta_kubernetes_*_node_name discovery labels); a collector's node comes from
// Collector.NodeName (pod.Spec.NodeName, captured by the collector watcher).
//
// Unassigned targets: targets that carry no node (GetNodeName == "") or whose
// node has no matching collector are placed via the fallback strategy when one
// is configured (SetFallbackStrategy("consistent-hashing")); otherwise they are
// retained in targetItems but assigned to no collector, re-evaluated on every
// collector change, and surfaced via the
// cloudwatch_agent_allocator_targets_unassigned gauge.
type perNodeAllocator struct {
	// m protects collectors, targetItems, targetItemsPerJobPerCollector,
	// collectorByNode and fallbackHasher for concurrent use.
	m sync.RWMutex

	// collectors is a map from a Collector's name to a Collector instance.
	collectors map[string]*Collector

	// targetItems is a map from a target item's hash to the target item.
	targetItems map[string]*target.Item

	// collectorKey -> job -> target item hash -> true
	targetItemsPerJobPerCollector map[string]map[string]map[string]bool

	// collectorByNode indexes collectors by their NodeName for O(1) placement.
	collectorByNode map[string]*Collector

	// fallbackHasher, when non-nil, is a consistent-hashing ring over all
	// collectors used to place targets that cannot be matched to a node.
	fallbackHasher *consistent.Consistent

	// warnedNoFallback ensures the "per-node has no fallback" warning is logged
	// at most once (guarded by the same lock as the allocation state).
	warnedNoFallback bool

	log logr.Logger

	filter Filter
}

func newPerNodeAllocator(log logr.Logger, opts ...AllocationOption) Allocator {
	pnAllocator := &perNodeAllocator{
		collectors:                    make(map[string]*Collector),
		targetItems:                   make(map[string]*target.Item),
		targetItemsPerJobPerCollector: make(map[string]map[string]map[string]bool),
		collectorByNode:               make(map[string]*Collector),
		log:                           log,
	}
	for _, opt := range opts {
		opt(pnAllocator)
	}
	return pnAllocator
}

// SetFilter sets the filtering hook to use.
func (pn *perNodeAllocator) SetFilter(filter Filter) {
	pn.filter = filter
}

// SetFallbackStrategy enables a fallback placement strategy for targets that
// cannot be matched to a node. Only "consistent-hashing" is supported; any other
// (or empty) name leaves the fallback disabled, so unmatched targets stay
// unassigned. Mirrors the upstream per-node strategy's optional fallbackStrategy.
func (pn *perNodeAllocator) SetFallbackStrategy(name string) {
	pn.m.Lock()
	defer pn.m.Unlock()
	if name != consistentHashingStrategyName {
		pn.log.Info("Unsupported fallback strategy for per-node, fallback disabled", "fallback", name)
		pn.fallbackHasher = nil
		return
	}
	cfg := consistent.Config{
		PartitionCount:    1061,
		ReplicationFactor: 5,
		Load:              1.1,
		Hasher:            hasher{},
	}
	pn.fallbackHasher = consistent.New(nil, cfg)
	// Seed the ring with any collectors already known.
	for _, c := range pn.collectors {
		pn.fallbackHasher.Add(c)
	}
	pn.log.Info("Per-node fallback strategy enabled", "fallback", name)
}

// addCollectorTargetItemMapping tracks which collector has which jobs and targets.
// The caller has to acquire a lock.
func (pn *perNodeAllocator) addCollectorTargetItemMapping(tg *target.Item) {
	if pn.targetItemsPerJobPerCollector[tg.CollectorName] == nil {
		pn.targetItemsPerJobPerCollector[tg.CollectorName] = make(map[string]map[string]bool)
	}
	if pn.targetItemsPerJobPerCollector[tg.CollectorName][tg.JobName] == nil {
		pn.targetItemsPerJobPerCollector[tg.CollectorName][tg.JobName] = make(map[string]bool)
	}
	pn.targetItemsPerJobPerCollector[tg.CollectorName][tg.JobName][tg.Hash()] = true
}

// addTargetToTargetItems assigns a target to the collector on the same node and
// stores it in targetItems. The caller has to acquire a lock. Targets with no
// resolvable node, or whose node has no matching collector, are placed via the
// consistent-hashing fallback if configured, otherwise left unassigned.
// It returns how the target was placed, for per-cycle log summaries.
func (pn *perNodeAllocator) addTargetToTargetItems(tg *target.Item) placement {
	// If this is a reassignment, decrement the previous collector's NumTargets.
	if previousCol, ok := pn.collectors[tg.CollectorName]; ok && tg.CollectorName != "" {
		previousCol.NumTargets--
		delete(pn.targetItemsPerJobPerCollector[tg.CollectorName][tg.JobName], tg.Hash())
		TargetsPerCollector.WithLabelValues(previousCol.String(), perNodeStrategyName).Set(float64(previousCol.NumTargets))
	}

	// Always keep the target in the pool so it can be (re)assigned later.
	tg.CollectorName = ""
	pn.targetItems[tg.Hash()] = tg

	nodeName := tg.GetNodeName()
	colOwner, ok := pn.collectorByNode[nodeName]
	if nodeName != "" && ok {
		tg.CollectorName = colOwner.Name
		pn.addCollectorTargetItemMapping(tg)
		colOwner.NumTargets++
		TargetsPerCollector.WithLabelValues(colOwner.String(), perNodeStrategyName).Set(float64(colOwner.NumTargets))
		pn.log.V(2).Info("per-node: assigned target to node-local collector",
			"target", strings.Join(tg.TargetURL, ","), "job", tg.JobName, "node", nodeName, "collector", colOwner.Name)
		return placedByNode
	}

	// No node, or no collector on that node. Use the consistent-hashing fallback
	// if configured; otherwise leave the target unassigned.
	if pn.fallbackHasher != nil && len(pn.collectors) > 0 {
		member := pn.fallbackHasher.LocateKey([]byte(strings.Join(tg.TargetURL, "")))
		if fallbackCol, exists := pn.collectors[member.String()]; exists {
			tg.CollectorName = fallbackCol.Name
			pn.addCollectorTargetItemMapping(tg)
			fallbackCol.NumTargets++
			TargetsPerCollector.WithLabelValues(fallbackCol.String(), perNodeStrategyName).Set(float64(fallbackCol.NumTargets))
			reason := "target has no node label"
			if nodeName != "" {
				reason = "no collector running on node " + nodeName
			}
			pn.log.V(1).Info("per-node: no node-local collector, used consistent-hashing fallback",
				"target", strings.Join(tg.TargetURL, ","), "job", tg.JobName, "node", nodeName, "collector", fallbackCol.Name, "reason", reason)
			return placedByFallback
		}
	}
	if pn.fallbackHasher == nil && !pn.warnedNoFallback {
		pn.warnedNoFallback = true
		pn.log.Info("per-node: no fallback strategy configured; targets that cannot be matched to a " +
			"node-local collector (e.g. targets with no node label) will be left UNASSIGNED and never " +
			"scraped. Configure a \"consistent-hashing\" fallback to place them.")
	}
	pn.log.V(1).Info("per-node: target left UNASSIGNED (no node-local collector and no usable fallback)",
		"target", strings.Join(tg.TargetURL, ","), "job", tg.JobName, "node", nodeName)
	return unplaced
}

// handleTargets reconciles added and removed targets against the current state.
func (pn *perNodeAllocator) handleTargets(diff diff.Changes[*target.Item]) {
	// Check for removals.
	for k, item := range pn.targetItems {
		if _, ok := diff.Removals()[k]; ok {
			if col, ok := pn.collectors[item.CollectorName]; ok && item.CollectorName != "" {
				col.NumTargets--
				delete(pn.targetItemsPerJobPerCollector[item.CollectorName][item.JobName], item.Hash())
				TargetsPerCollector.WithLabelValues(item.CollectorName, perNodeStrategyName).Set(float64(col.NumTargets))
			}
			delete(pn.targetItems, k)
		}
	}

	// Check for additions.
	var byNode, byFallback, unassigned, added int
	for k, item := range diff.Additions() {
		if _, ok := pn.targetItems[k]; ok {
			continue
		}
		added++
		switch pn.addTargetToTargetItems(item) {
		case placedByNode:
			byNode++
		case placedByFallback:
			byFallback++
		case unplaced:
			unassigned++
		}
	}

	if added > 0 || len(diff.Removals()) > 0 {
		pn.log.Info("per-node: target reconcile complete",
			"added", added, "removed", len(diff.Removals()),
			"assigned_by_node", byNode, "assigned_by_fallback", byFallback, "unassigned", unassigned,
			"total_targets", len(pn.targetItems), "collectors", len(pn.collectors))
	}

	pn.recordUnassigned()
}

// handleCollectors reconciles added and removed collectors, rebuilds the node
// index and re-allocates all known targets.
func (pn *perNodeAllocator) handleCollectors(diff diff.Changes[*Collector]) {
	// Clear removed collectors.
	for _, k := range diff.Removals() {
		delete(pn.collectors, k.Name)
		delete(pn.targetItemsPerJobPerCollector, k.Name)
		if pn.fallbackHasher != nil {
			pn.fallbackHasher.Remove(k.Name)
		}
		TargetsPerCollector.WithLabelValues(k.Name, perNodeStrategyName).Set(0)
	}
	// Insert the new collectors.
	for _, i := range diff.Additions() {
		pn.collectors[i.Name] = NewCollector(i.Name, i.NodeName)
		if pn.fallbackHasher != nil {
			pn.fallbackHasher.Add(pn.collectors[i.Name])
		}
	}

	// Rebuild the node index from the current collector set.
	pn.collectorByNode = make(map[string]*Collector)
	for _, c := range pn.collectors {
		if c.NodeName == "" {
			continue
		}
		// Deterministic tie-break: normally there is one collector (DaemonSet pod)
		// per node, but a maxSurge rollout can briefly place two pods on the same
		// node. Keep the one with the smaller pod name so node ownership — and thus
		// target placement — doesn't flap with map iteration order.
		if existing, ok := pn.collectorByNode[c.NodeName]; ok && existing.Name <= c.Name {
			continue
		}
		pn.collectorByNode[c.NodeName] = c
	}

	// Log the node->collector index so it's clear which node each agent owns.
	mapping := make([]string, 0, len(pn.collectorByNode))
	for node, c := range pn.collectorByNode {
		mapping = append(mapping, node+"="+c.Name)
	}
	sort.Strings(mapping)
	noNode := len(pn.collectors) - len(pn.collectorByNode)
	pn.log.Info("per-node: collector node index rebuilt",
		"collectors", len(pn.collectors), "nodes_indexed", len(pn.collectorByNode),
		"collectors_without_node", noNode, "mapping", strings.Join(mapping, ","))

	// Re-allocate all targets against the new collector set.
	var byNode, byFallback, unassigned int
	for _, item := range pn.targetItems {
		switch pn.addTargetToTargetItems(item) {
		case placedByNode:
			byNode++
		case placedByFallback:
			byFallback++
		case unplaced:
			unassigned++
		}
	}
	pn.log.Info("per-node: re-allocated all targets after collector change",
		"added_collectors", len(diff.Additions()), "removed_collectors", len(diff.Removals()),
		"assigned_by_node", byNode, "assigned_by_fallback", byFallback, "unassigned", unassigned,
		"total_targets", len(pn.targetItems))

	pn.recordUnassigned()
}

// recordUnassigned updates the unassigned-targets gauge. Caller must hold the lock.
func (pn *perNodeAllocator) recordUnassigned() {
	var count float64
	for _, item := range pn.targetItems {
		if item.CollectorName == "" {
			count++
		}
	}
	targetsUnassigned.Set(count)
}

// SetTargets accepts a list of targets that will be used to make load balancing
// decisions. This method should be called when there are new targets discovered
// or existing targets are shutdown.
func (pn *perNodeAllocator) SetTargets(targets map[string]*target.Item) {
	timer := prometheus.NewTimer(TimeToAssign.WithLabelValues("SetTargets", perNodeStrategyName))
	defer timer.ObserveDuration()

	if pn.filter != nil {
		targets = pn.filter.Apply(targets)
	}
	RecordTargetsKept(targets)

	pn.m.Lock()
	defer pn.m.Unlock()

	// If there are no collectors, just track the targets so they can be assigned
	// once collectors appear.
	if len(pn.collectors) == 0 {
		pn.log.Info("No collector instances present, saving targets to allocate to collector(s)")
		targetsDiff := diff.Maps(pn.targetItems, targets)
		for k, item := range targetsDiff.Additions() {
			if _, ok := pn.targetItems[k]; !ok {
				pn.targetItems[k] = item
			}
		}
		for k := range targetsDiff.Removals() {
			delete(pn.targetItems, k)
		}
		pn.recordUnassigned()
		return
	}

	// Check for target changes.
	targetsDiff := diff.Maps(pn.targetItems, targets)
	if len(targetsDiff.Additions()) != 0 || len(targetsDiff.Removals()) != 0 {
		pn.handleTargets(targetsDiff)
	}
}

// SetCollectors sets the set of collectors with key=collectorName, value=Collector object.
// This method is called when Collectors are added or removed.
func (pn *perNodeAllocator) SetCollectors(collectors map[string]*Collector) {
	timer := prometheus.NewTimer(TimeToAssign.WithLabelValues("SetCollectors", perNodeStrategyName))
	defer timer.ObserveDuration()

	CollectorsAllocatable.WithLabelValues(perNodeStrategyName).Set(float64(len(collectors)))
	if len(collectors) == 0 {
		pn.log.Info("No collector instances present")
		return
	}

	pn.m.Lock()
	defer pn.m.Unlock()

	// Check for collector changes.
	collectorsDiff := diff.Maps(pn.collectors, collectors)
	if len(collectorsDiff.Additions()) != 0 || len(collectorsDiff.Removals()) != 0 {
		pn.handleCollectors(collectorsDiff)
	}
	pn.log.Info("Setting collector completed")
}

func (pn *perNodeAllocator) GetTargetsForCollectorAndJob(collector string, job string) []*target.Item {
	pn.m.RLock()
	defer pn.m.RUnlock()
	if _, ok := pn.targetItemsPerJobPerCollector[collector]; !ok {
		return []*target.Item{}
	}
	if _, ok := pn.targetItemsPerJobPerCollector[collector][job]; !ok {
		return []*target.Item{}
	}
	targetItemsCopy := make([]*target.Item, len(pn.targetItemsPerJobPerCollector[collector][job]))
	index := 0
	for targetHash := range pn.targetItemsPerJobPerCollector[collector][job] {
		targetItemsCopy[index] = pn.targetItems[targetHash]
		index++
	}
	return targetItemsCopy
}

// TargetItems returns a shallow copy of the targetItems map.
func (pn *perNodeAllocator) TargetItems() map[string]*target.Item {
	pn.m.RLock()
	defer pn.m.RUnlock()
	targetItemsCopy := make(map[string]*target.Item)
	for k, v := range pn.targetItems {
		targetItemsCopy[k] = v
	}
	return targetItemsCopy
}

// Collectors returns a shallow copy of the collectors map.
func (pn *perNodeAllocator) Collectors() map[string]*Collector {
	pn.m.RLock()
	defer pn.m.RUnlock()
	collectorsCopy := make(map[string]*Collector)
	for k, v := range pn.collectors {
		collectorsCopy[k] = v
	}
	return collectorsCopy
}
