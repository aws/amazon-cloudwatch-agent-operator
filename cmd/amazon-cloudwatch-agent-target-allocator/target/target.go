// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"fmt"
	"net/url"

	"github.com/prometheus/common/model"
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
