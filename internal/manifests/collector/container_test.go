// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVolumeMounts(t *testing.T) {
	volumeMount := getVolumeMounts("windows")
	assert.Equal(t, volumeMount.MountPath, "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\cwagentconfig")

	volumeMount = getVolumeMounts("linux")
	assert.Equal(t, volumeMount.MountPath, "/etc/cwagentconfig")

	volumeMount = getVolumeMounts("")
	assert.Equal(t, volumeMount.MountPath, "/etc/cwagentconfig")
}

func TestGetPrometheusVolumeMounts(t *testing.T) {
	volumeMount := getPrometheusVolumeMounts("windows")
	assert.Equal(t, volumeMount.MountPath, "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\prometheusconfig")

	volumeMount = getPrometheusVolumeMounts("linux")
	assert.Equal(t, volumeMount.MountPath, "/etc/prometheusconfig")

	volumeMount = getPrometheusVolumeMounts("")
	assert.Equal(t, volumeMount.MountPath, "/etc/prometheusconfig")
}
