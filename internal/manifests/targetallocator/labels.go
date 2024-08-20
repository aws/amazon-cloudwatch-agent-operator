// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// Labels return the common labels to all TargetAllocator objects that are part of a managed AmazonCloudWatchAgent.
func Labels(instance v1alpha1.AmazonCloudWatchAgent, name string) map[string]string {
	// new map every time, so that we don't touch the instance's label
	base := map[string]string{}
	if nil != instance.Labels {
		for k, v := range instance.Labels {
			base[k] = v
		}
	}

	base["app.kubernetes.io/managed-by"] = "amazon-cloudwatch-agent-operator"
	base["app.kubernetes.io/instance"] = naming.Truncate("%s.%s", 63, instance.Namespace, instance.Name)
	base["app.kubernetes.io/part-of"] = "amazon-cloudwatch-agent"
	base["app.kubernetes.io/component"] = "amazon-cloudwatch-agent-targetallocator"

	if _, ok := base["app.kubernetes.io/name"]; !ok {
		base["app.kubernetes.io/name"] = name
	}

	return base
}
