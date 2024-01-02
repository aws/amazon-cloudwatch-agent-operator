// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

func upgrade0_15_0(u VersionUpgrade, otelcol *v1alpha1.AmazonCloudWatchAgent) (*v1alpha1.AmazonCloudWatchAgent, error) {
	delete(otelcol.Spec.Args, "--new-metrics")
	delete(otelcol.Spec.Args, "--legacy-metrics")
	existing := &corev1.ConfigMap{}
	updated := existing.DeepCopy()
	u.Recorder.Event(updated, "Normal", "Upgrade", "upgrade to v0.15.0 dropped the deprecated metrics arguments")

	return otelcol, nil
}
