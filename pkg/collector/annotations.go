// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"crypto/sha256"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// Annotations return the annotations for AmazonCloudWatchAgent pod.
func Annotations(instance v1alpha1.AmazonCloudWatchAgent) map[string]string {
	annotations := map[string]string{}

	// make sure sha256 for configMap is always calculated
	annotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(instance.Spec.Config)

	return annotations
}

// PodAnnotations return the spec annotations for AmazonCloudWatchAgent pod.
func PodAnnotations(instance v1alpha1.AmazonCloudWatchAgent) map[string]string {
	// new map every time, so that we don't touch the instance's annotations
	podAnnotations := map[string]string{}

	// allow override of pod annotations
	for k, v := range instance.Spec.PodAnnotations {
		podAnnotations[k] = v
	}

	// propagating annotations from metadata.annotations
	for kMeta, vMeta := range Annotations(instance) {
		if _, found := podAnnotations[kMeta]; !found {
			podAnnotations[kMeta] = vMeta
		}
	}

	// make sure sha256 for configMap is always calculated
	podAnnotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(instance.Spec.Config)

	return podAnnotations
}

func getConfigMapSHA(config string) string {
	h := sha256.Sum256([]byte(config))
	return fmt.Sprintf("%x", h)
}
