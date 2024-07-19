// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

const (
	ComponentAmazonCloudWatchAgent = "amazon-cloudwatch-agent"
)

// Build creates the manifest for the collector resource.
func Build(params manifests.Params) ([]client.Object, error) {
	var resourceManifests []client.Object
	var manifestFactories []manifests.K8sManifestFactory
	switch params.OtelCol.Spec.Mode {
	case v1alpha1.ModeDeployment:
		manifestFactories = append(manifestFactories, manifests.Factory(Deployment))
		manifestFactories = append(manifestFactories, manifests.Factory(PodDisruptionBudget))
	case v1alpha1.ModeStatefulSet:
		manifestFactories = append(manifestFactories, manifests.Factory(StatefulSet))
		manifestFactories = append(manifestFactories, manifests.Factory(PodDisruptionBudget))
	case v1alpha1.ModeDaemonSet:
		manifestFactories = append(manifestFactories, manifests.Factory(DaemonSet))
	case v1alpha1.ModeSidecar:
		params.Log.V(5).Info("not building sidecar...")
	}
	manifestFactories = append(manifestFactories, []manifests.K8sManifestFactory{
		manifests.Factory(ConfigMap),
		manifests.Factory(HorizontalPodAutoscaler),
		manifests.Factory(ServiceAccount),
		manifests.Factory(Service),
		manifests.Factory(HeadlessService),
		manifests.Factory(MonitoringService),
		manifests.Factory(Ingress),
	}...)

	for _, factory := range manifestFactories {
		res, err := factory(params)
		if err != nil {
			return nil, err
		} else if manifests.ObjectIsNotNil(res) {
			resourceManifests = append(resourceManifests, res)
		}
	}
	routes, err := Routes(params)
	if err != nil {
		return nil, err
	}
	// NOTE: we cannot just unpack the slice, the type checker doesn't coerce the type correctly.
	for _, route := range routes {
		resourceManifests = append(resourceManifests, route)
	}
	return resourceManifests, nil
}
