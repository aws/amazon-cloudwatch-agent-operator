// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crdmanager

import (
	_ "embed"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// BundledVersion is the prometheus-operator release the embedded ServiceMonitor
// and PodMonitor CRDs are pinned to. Keep this in sync with the asset files under
// assets/ and with the prometheus-operator API types vendored by the target
// allocator, so the schema the operator installs matches the schema the target
// allocator knows how to read.
const BundledVersion = "0.92.0"

//go:embed assets/monitoring.coreos.com_servicemonitors.yaml
var serviceMonitorCRD []byte

//go:embed assets/monitoring.coreos.com_podmonitors.yaml
var podMonitorCRD []byte

// bundledCRDs decodes the embedded CRD manifests into typed objects.
func bundledCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	out := make([]*apiextensionsv1.CustomResourceDefinition, 0, 2)
	for _, raw := range [][]byte{serviceMonitorCRD, podMonitorCRD} {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(raw, crd); err != nil {
			return nil, fmt.Errorf("decode embedded CRD: %w", err)
		}
		out = append(out, crd)
	}
	return out, nil
}
