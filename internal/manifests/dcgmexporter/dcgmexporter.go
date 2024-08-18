// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

const (
	ComponentDcgmExporter = "dcgm-exporter"
)

// Build creates the manifest for the exporter resource.
func Build(params manifests.Params) ([]client.Object, error) {
	var resourceManifests []client.Object
	var manifestFactories []manifests.K8sManifestFactory[manifests.Params]
	manifestFactories = append(manifestFactories, []manifests.K8sManifestFactory[manifests.Params]{
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
