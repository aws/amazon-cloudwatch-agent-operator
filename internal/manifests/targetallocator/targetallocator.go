// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

// Build creates the manifest for the TargetAllocator resource.
func Build(params manifests.Params) ([]client.Object, error) {
	var resourceManifests []client.Object
	if !params.OtelCol.Spec.TargetAllocator.Enabled {
		return resourceManifests, nil
	}
	resourceFactories := []manifests.K8sManifestFactory{
		manifests.Factory(ConfigMap),
		manifests.Factory(Deployment),
		manifests.FactoryWithoutError(ServiceAccount),
		manifests.FactoryWithoutError(Service),
	}
	for _, factory := range resourceFactories {
		res, err := factory(params)
		if err != nil {
			return nil, err
		} else if manifests.ObjectIsNotNil(res) {
			resourceManifests = append(resourceManifests, res)
		}
	}
	return resourceManifests, nil
}
