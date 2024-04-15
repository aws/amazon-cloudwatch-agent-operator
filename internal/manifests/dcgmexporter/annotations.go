// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"crypto/sha256"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// Annotations return the annotations for DcgmExporter pod.
func Annotations(instance v1alpha1.DcgmExporter) map[string]string {
	// new map every time, so that we don't touch the instance's annotations
	annotations := map[string]string{}

	annotations["k8s-app"] = ComponentDcgmExporter

	// allow override of prometheus annotations
	if nil != instance.Annotations {
		for k, v := range instance.Annotations {
			annotations[k] = v
		}
	}
	// make sure sha256 for configMap is always calculated
	annotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(instance.Spec.MetricsConfig)

	return annotations
}

func getConfigMapSHA(config string) string {
	h := sha256.Sum256([]byte(config))
	return fmt.Sprintf("%x", h)
}
