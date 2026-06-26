// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"errors"
	"fmt"

	"github.com/buraksezer/consistent"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/target"
)

type AllocatorProvider func(log logr.Logger, opts ...AllocationOption) Allocator

var (
	registry = map[string]AllocatorProvider{}

	// TargetsPerCollector records how many targets have been assigned to each collector.
	// It is currently the responsibility of the strategy to track this information.
	TargetsPerCollector = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cloudwatch_agent_allocator_targets_per_collector",
		Help: "The number of targets for each collector.",
	}, []string{"collector_name", "strategy"})
	CollectorsAllocatable = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cloudwatch_agent_allocator_collectors_allocatable",
		Help: "Number of collectors the allocator is able to allocate to.",
	}, []string{"strategy"})
	TimeToAssign = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "cloudwatch_agent_allocator_time_to_allocate",
		Help: "The time it takes to allocate",
	}, []string{"method", "strategy"})
	targetsRemaining = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloudwatch_agent_allocator_targets_remaining",
		Help: "Number of targets kept after filtering.",
	})
	// targetsUnassigned records targets that could not be placed on any collector
	// (e.g. per-node strategy: target has no node label, or no collector runs on
	// the target's node). Such targets are retained and re-evaluated on the next
	// collector change, but are not scraped until they can be assigned.
	targetsUnassigned = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloudwatch_agent_allocator_targets_unassigned",
		Help: "Number of targets that could not be assigned to a collector.",
	})
)

type AllocationOption func(Allocator)

type Filter interface {
	Apply(map[string]*target.Item) map[string]*target.Item
}

func WithFilter(filter Filter) AllocationOption {
	return func(allocator Allocator) {
		allocator.SetFilter(filter)
	}
}

// fallbackStrategySetter is implemented by allocators that support a fallback
// placement strategy for targets the primary strategy cannot assign.
type fallbackStrategySetter interface {
	SetFallbackStrategy(name string)
}

// WithFallbackStrategy configures a fallback placement strategy by name. It is a
// no-op for an empty name or for allocators that don't support a fallback
// (only per-node does today).
func WithFallbackStrategy(name string) AllocationOption {
	return func(allocator Allocator) {
		if name == "" {
			return
		}
		if setter, ok := allocator.(fallbackStrategySetter); ok {
			setter.SetFallbackStrategy(name)
		}
	}
}

func RecordTargetsKept(targets map[string]*target.Item) {
	targetsRemaining.Add(float64(len(targets)))
}

func New(name string, log logr.Logger, opts ...AllocationOption) (Allocator, error) {
	if p, ok := registry[name]; ok {
		return p(log.WithValues("allocator", name), opts...), nil
	}
	return nil, fmt.Errorf("unregistered strategy: %s", name)
}

func Register(name string, provider AllocatorProvider) error {
	if _, ok := registry[name]; ok {
		return errors.New("already registered")
	}
	registry[name] = provider
	return nil
}

func GetRegisteredAllocatorNames() []string {
	var names []string
	for s := range registry {
		names = append(names, s)
	}
	return names
}

type Allocator interface {
	SetCollectors(collectors map[string]*Collector)
	SetTargets(targets map[string]*target.Item)
	TargetItems() map[string]*target.Item
	Collectors() map[string]*Collector
	GetTargetsForCollectorAndJob(collector string, job string) []*target.Item
	SetFilter(filter Filter)
}

var _ consistent.Member = Collector{}

// Collector Creates a struct that holds Collector information.
// This struct will be parsed into endpoint with Collector and jobs info.
// NodeName is the Kubernetes node the collector pod runs on; it is used by the
// per-node allocation strategy to match targets to the collector on their node.
// This struct can be extended with information like annotations and labels in the future.
type Collector struct {
	Name       string
	NodeName   string
	NumTargets int
}

func (c Collector) Hash() string {
	return c.Name
}

func (c Collector) String() string {
	return c.Name
}

func NewCollector(name, node string) *Collector {
	return &Collector{Name: name, NodeName: node}
}

func init() {
	err := Register(consistentHashingStrategyName, newConsistentHashingAllocator)
	if err != nil {
		panic(err)
	}
	err = Register(perNodeStrategyName, newPerNodeAllocator)
	if err != nil {
		panic(err)
	}
}
