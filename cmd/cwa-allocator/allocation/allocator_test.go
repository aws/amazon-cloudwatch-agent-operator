// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"fmt"
	"strconv"

	"github.com/prometheus/common/model"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/cwa-allocator/target"
)

func colIndex(index, numCols int) int {
	if numCols == 0 {
		return -1
	}
	return index % numCols
}

func MakeNNewTargets(n int, numCollectors int, startingIndex int) map[string]*target.Item {
	toReturn := map[string]*target.Item{}
	for i := startingIndex; i < n+startingIndex; i++ {
		collector := fmt.Sprintf("collector-%d", colIndex(i, numCollectors))
		label := model.LabelSet{
			"collector": model.LabelValue(collector),
			"i":         model.LabelValue(strconv.Itoa(i)),
			"total":     model.LabelValue(strconv.Itoa(n + startingIndex)),
		}
		newTarget := target.NewItem(fmt.Sprintf("test-job-%d", i), fmt.Sprintf("test-url-%d", i), label, collector)
		toReturn[newTarget.Hash()] = newTarget
	}
	return toReturn
}

func MakeNCollectors(n int, startingIndex int) map[string]*Collector {
	toReturn := map[string]*Collector{}
	for i := startingIndex; i < n+startingIndex; i++ {
		collector := fmt.Sprintf("collector-%d", i)
		toReturn[collector] = &Collector{
			Name:       collector,
			NumTargets: 0,
		}
	}
	return toReturn
}

func MakeNNewTargetsWithEmptyCollectors(n int, startingIndex int) map[string]*target.Item {
	toReturn := map[string]*target.Item{}
	for i := startingIndex; i < n+startingIndex; i++ {
		label := model.LabelSet{
			"i":     model.LabelValue(strconv.Itoa(i)),
			"total": model.LabelValue(strconv.Itoa(n + startingIndex)),
		}
		newTarget := target.NewItem(fmt.Sprintf("test-job-%d", i), fmt.Sprintf("test-url-%d", i), label, "")
		toReturn[newTarget.Hash()] = newTarget
	}
	return toReturn
}
