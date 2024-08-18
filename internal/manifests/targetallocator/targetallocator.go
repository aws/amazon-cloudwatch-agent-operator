// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
)

const (
	ComponentAmazonCloudWatchAgentTargetAllocator = "amazon-cloudwatch-agent-targetallocator"
)

// Build creates the manifest for the TargetAllocator resource.
func Build(params Params) ([]client.Object, error) {
	var resourceManifests []client.Object
	resourceFactories := []manifests.K8sManifestFactory[Params]{
		manifests.Factory(ConfigMap),
		manifests.Factory(Deployment),
		manifests.FactoryWithoutError(ServiceAccount),
		manifests.FactoryWithoutError(Service),
		manifests.Factory(PodDisruptionBudget),
	}

	if params.TargetAllocator.Spec.Observability.Metrics.EnableMetrics && featuregate.PrometheusOperatorIsAvailable.IsEnabled() {
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

type Params struct {
	Client          client.Client
	Recorder        record.EventRecorder
	Scheme          *runtime.Scheme
	Log             logr.Logger
	Collector       v1alpha1.AmazonCloudWatchAgent
	TargetAllocator v1alpha1.TargetAllocator
	Config          config.Config
}
