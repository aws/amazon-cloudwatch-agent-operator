// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
)

const (
	ComponentOpenTelemetryTargetAllocator = "opentelemetry-targetallocator"
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
		manifests.Factory(PodDisruptionBudget),
	}

	if params.OtelCol.Spec.TargetAllocator.Observability.Metrics.EnableMetrics && featuregate.PrometheusOperatorIsAvailable.IsEnabled() {
		resourceFactories = append(resourceFactories, manifests.FactoryWithoutError(ServiceMonitor))
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
