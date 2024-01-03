// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// nolint unused
func noop(cl client.Client, otelcol *v1alpha1.AmazonCloudWatchAgent) (*v1alpha1.AmazonCloudWatchAgent, error) {
	return otelcol, nil
}
