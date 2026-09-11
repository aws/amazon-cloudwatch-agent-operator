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
	// new map every time, so that we don't touch the instance's annotations
	annotations := map[string]string{}

	// do not set default prometheus annotations as the cw agent does not expose a prometheus metrics endpoint
	// annotations["prometheus.io/scrape"] = "true"
	// annotations["prometheus.io/port"] = "8888"
	// annotations["prometheus.io/path"] = "/metrics"

	// allow override of annotations
	if nil != instance.Annotations {
		for k, v := range instance.Annotations {
			annotations[k] = v
		}
	}

	// make sure sha256 for configMap is always calculated
	annotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(configHashInput(instance))

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
	podAnnotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(configHashInput(instance))

	return podAnnotations
}

// configHashInput returns the combined config string used for the pod-template restart hash.
func configHashInput(instance v1alpha1.AmazonCloudWatchAgent) string {
	config := instance.Spec.Config
	if !instance.Spec.Prometheus.IsEmpty() {
		promYaml, err := instance.Spec.Prometheus.Yaml()
		if err != nil {
			// Unreachable from the CRD path: Config.Object only ever holds JSON-decoded
			// types, all of which gopkg.in/yaml.v3 encodes. Deliberately a constant: two
			// different unserializable specs hash equal, so a Prometheus-only change would
			// not roll pods while the error persists. Hash stability across operator
			// restarts depends on gopkg.in/yaml.v3 (v3.0.1) sorting map keys
			// (encode.go:189, and :242 for the inline-map branch).
			config += "\x00prometheus-serialize-error"
		} else {
			config += "\x00" + promYaml // null byte prevents collision between config suffix and promYAML prefix
		}
	}
	return config
}

func getConfigMapSHA(config string) string {
	h := sha256.Sum256([]byte(config))
	return fmt.Sprintf("%x", h)
}
