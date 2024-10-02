// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/open-o11y/prometheus-sample-app/metrics"
)

func main() {

	cmd := metrics.CommandLine{}
	cmd.Run()
}
