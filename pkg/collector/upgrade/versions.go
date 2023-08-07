// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"github.com/Masterminds/semver/v3"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

type upgradeFunc func(u VersionUpgrade, otelcol *v1alpha1.AmazonCloudWatchAgent) (*v1alpha1.AmazonCloudWatchAgent, error)

type otelcolVersion struct {
	upgrade upgradeFunc
	semver.Version
}

var (
	versions []otelcolVersion

	// Latest represents the latest version that we need to upgrade. This is not necessarily the latest known version.
	Latest = versions[len(versions)-1]
)
