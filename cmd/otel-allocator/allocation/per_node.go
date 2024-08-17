// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/otel-allocator/target"
)

const perNodeStrategyName = "per-node"

var _ Strategy = &perNodeStrategy{}

type perNodeStrategy struct {
	collectorByNode map[string]*Collector
}

func newPerNodeStrategy() Strategy {
	return &perNodeStrategy{
		collectorByNode: make(map[string]*Collector),
	}
}

func (s *perNodeStrategy) GetName() string {
	return perNodeStrategyName
}

func (s *perNodeStrategy) GetCollectorForTarget(collectors map[string]*Collector, item *target.Item) (*Collector, error) {
	targetNodeName := item.GetNodeName()
	collector, ok := s.collectorByNode[targetNodeName]
	if !ok {
		return nil, fmt.Errorf("could not find collector for node %s", targetNodeName)
	}
	return collectors[collector.Name], nil
}

func (s *perNodeStrategy) SetCollectors(collectors map[string]*Collector) {
	clear(s.collectorByNode)
	for _, collector := range collectors {
		if collector.NodeName != "" {
			s.collectorByNode[collector.NodeName] = collector
		}
	}
}
