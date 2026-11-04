// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeName(t *testing.T) {
	tests := []struct {
		name   string
		labels model.LabelSet
		want   string
	}{
		{
			name:   "pod node label",
			labels: model.LabelSet{nodeNameLabelPod: "node-a"},
			want:   "node-a",
		},
		{
			name:   "node label",
			labels: model.LabelSet{nodeNameLabelNode: "node-b"},
			want:   "node-b",
		},
		{
			name:   "endpoint node label",
			labels: model.LabelSet{nodeNameLabelEndpoint: "node-c"},
			want:   "node-c",
		},
		{
			name: "pod label takes precedence over others",
			labels: model.LabelSet{
				nodeNameLabelPod:      "node-pod",
				nodeNameLabelNode:     "node-node",
				nodeNameLabelEndpoint: "node-endpoint",
			},
			want: "node-pod",
		},
		{
			name: "endpointslice target kind Node resolves to target name",
			labels: model.LabelSet{
				endpointSliceTargetKindLabel: "Node",
				endpointSliceTargetNameLabel: "node-d",
			},
			want: "node-d",
		},
		{
			name: "endpointslice target kind Pod is not a node",
			labels: model.LabelSet{
				endpointSliceTargetKindLabel: "Pod",
				endpointSliceTargetNameLabel: "some-pod",
			},
			want: "",
		},
		{
			name:   "no node labels",
			labels: model.LabelSet{"__meta_kubernetes_namespace": "test"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := NewItem("job", "10.0.0.1:8080", tt.labels, "")
			assert.Equal(t, tt.want, item.GetNodeName())
		})
	}
}
