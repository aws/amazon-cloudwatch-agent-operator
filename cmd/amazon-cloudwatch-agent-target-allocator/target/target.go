// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"fmt"
	"net/url"

	"github.com/prometheus/common/model"
)

// nodeLabels are the discovery meta-labels that identify the node a target
// resides on. They mirror the upstream OpenTelemetry target allocator's
// per-node node-label set. See:
// https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
const (
	nodeNameLabelPod      model.LabelName = "__meta_kubernetes_pod_node_name"
	nodeNameLabelNode     model.LabelName = "__meta_kubernetes_node_name"
	nodeNameLabelEndpoint model.LabelName = "__meta_kubernetes_endpoint_node_name"

	endpointSliceTargetKindLabel model.LabelName = "__meta_kubernetes_endpointslice_address_target_kind"
	endpointSliceTargetNameLabel model.LabelName = "__meta_kubernetes_endpointslice_address_target_name"
)

// LinkJSON This package contains common structs and methods that relate to scrape targets.
type LinkJSON struct {
	Link string `json:"_link"`
}

type Item struct {
	JobName       string         `json:"-"`
	Link          LinkJSON       `json:"-"`
	TargetURL     []string       `json:"targets"`
	Labels        model.LabelSet `json:"labels"`
	CollectorName string         `json:"-"`
	hash          string
}

func (t *Item) Hash() string {
	return t.hash
}

// GetNodeName returns the Kubernetes node a target resides on, derived from its
// service-discovery meta labels. Pod targets (PodMonitor, role: pod) always carry
// a node; endpoint targets (ServiceMonitor, role: endpoints/endpointslice) only
// carry one when the endpoint is backed by a pod on a node. Returns "" when no
// node can be determined (e.g. non-pod / external endpoints), in which case the
// per-node strategy leaves the target unassigned.
func (t *Item) GetNodeName() string {
	for _, labelName := range []model.LabelName{nodeNameLabelPod, nodeNameLabelNode, nodeNameLabelEndpoint} {
		if val := t.Labels[labelName]; val != "" {
			return string(val)
		}
	}

	if t.Labels[endpointSliceTargetKindLabel] != "Node" {
		return ""
	}
	return string(t.Labels[endpointSliceTargetNameLabel])
}

// NewItem Creates a new target item.
// INVARIANTS:
// * Item fields must not be modified after creation.
// * Item should only be made via its constructor, never directly.
func NewItem(jobName string, targetURL string, label model.LabelSet, collectorName string) *Item {
	return &Item{
		JobName:       jobName,
		Link:          LinkJSON{Link: fmt.Sprintf("/jobs/%s/targets", url.QueryEscape(jobName))},
		hash:          jobName + targetURL + label.Fingerprint().String(),
		TargetURL:     []string{targetURL},
		Labels:        label,
		CollectorName: collectorName,
	}
}
