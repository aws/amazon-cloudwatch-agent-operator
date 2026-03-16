// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

const (
	ComponentNodeExporter = "node-exporter"
)

// Build creates the manifest for the node exporter resource.
func Build(params manifests.Params) ([]client.Object, error) {
	var resourceManifests []client.Object
	var manifestFactories []manifests.K8sManifestFactory
	manifestFactories = append(manifestFactories, []manifests.K8sManifestFactory{
		manifests.FactoryWithoutError(DaemonSet),
		manifests.Factory(ConfigMap),
		manifests.FactoryWithoutError(ServiceAccount),
		manifests.Factory(Service),
	}...)
	for _, factory := range manifestFactories {
		res, err := factory(params)
		if err != nil {
			return nil, err
		} else if manifests.ObjectIsNotNil(res) {
			resourceManifests = append(resourceManifests, res)
		}
	}
	return resourceManifests, nil
}
