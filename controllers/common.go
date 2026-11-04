// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator"
)

const (
	acceleratedComputeMetrics = "accelerated_compute_metrics"
)

func isNamespaceScoped(obj client.Object) bool {
	switch obj.(type) {
	case *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding:
		return false
	default:
		return true
	}
}

// BuildCollector returns the generation and collected errors of all manifests for a given instance.
func BuildCollector(params manifests.Params) ([]client.Object, error) {
	builders := []manifests.Builder{
		collector.Build,
		targetallocator.Build,
	}
	var resources []client.Object
	for _, builder := range builders {
		objs, err := builder(params)
		if err != nil {
			return nil, err
		}
		resources = append(resources, objs...)
	}
	return resources, nil
}
func reconcileDesiredObjectUIDs(ctx context.Context, kubeClient client.Client, logger logr.Logger,
	owner metav1.Object, scheme *runtime.Scheme, desiredObjects ...client.Object) (map[types.UID]client.Object, error) {
	var errs []error
	existingObjectMap := make(map[types.UID]client.Object)
	var existingObjectList []client.Object

	for _, desired := range desiredObjects {
		l := logger.WithValues(
			"object_name", desired.GetName(),
			"object_kind", desired.GetObjectKind(),
		)
		if isNamespaceScoped(desired) {
			if setErr := ctrl.SetControllerReference(owner, desired, scheme); setErr != nil {
				l.Error(setErr, "failed to set controller owner reference to desired")
				errs = append(errs, setErr)
				continue
			}
		}

		// existing is an object the controller runtime will hydrate for us
		// we obtain the existing object by deep copying the desired object because it's the most convenient way
		existing := desired.DeepCopyObject().(client.Object)
		existingObjectList = append(existingObjectList, existing) //uid are not assigned yet

		mutateFn := manifests.MutateFuncFor(existing, desired)
		var op controllerutil.OperationResult
		crudErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			result, createOrUpdateErr := ctrl.CreateOrUpdate(ctx, kubeClient, existing, mutateFn)
			op = result
			return createOrUpdateErr
		})
		if crudErr != nil && errors.Is(crudErr, manifests.ErrImmutableChange) {
			l.Error(crudErr, "detected immutable field change, trying to delete, new object will be created on next reconcile", "existing", existing.GetName())
			delErr := kubeClient.Delete(ctx, existing)
			if delErr != nil {
				return nil, delErr
			}
			continue
		} else if crudErr != nil {
			l.Error(crudErr, "failed to configure desired")
			errs = append(errs, crudErr)
			continue
		}

		l.V(1).Info(fmt.Sprintf("desired has been %s", op))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to create objects for %s: %w", owner.GetName(), errors.Join(errs...))
	}
	for _, obj := range existingObjectList {
		existingObjectMap[obj.GetUID()] = obj
	}
	return existingObjectMap, nil
}

func reconcileDesiredObjectsWPrune(ctx context.Context, kubeClient client.Client, logger logr.Logger, owner v1alpha1.AmazonCloudWatchAgent, scheme *runtime.Scheme,
	desiredObjects []client.Object,
	searchOwnedObjectsFunc func(ctx context.Context, owner v1alpha1.AmazonCloudWatchAgent) (map[types.UID]client.Object, error),
) error {
	previouslyOwnedObjects, err := searchOwnedObjectsFunc(ctx, owner)
	if err != nil {
		return fmt.Errorf("failed to search owned objects: %w", err)
	}

	desiredObjectMap, err := reconcileDesiredObjectUIDs(ctx, kubeClient, logger, &owner, scheme, desiredObjects...)
	if err != nil {
		return fmt.Errorf("failed to reconcile desired objects: %w", err)
	}

	// Pruning owned objects in the cluster which are not should not be present after the reconciliation.
	err = pruneStaleObjects(ctx, kubeClient, logger, previouslyOwnedObjects, desiredObjectMap)
	if err != nil {
		return fmt.Errorf("failed to prune objects for %s: %w", owner.GetName(), err)
	}
	return nil
}

// reconcileDesiredObjects runs the reconcile process using the mutateFn over the given list of objects.
func reconcileDesiredObjects(ctx context.Context, kubeClient client.Client, logger logr.Logger, owner metav1.Object, scheme *runtime.Scheme, desiredObjects ...client.Object) error {
	_, err := reconcileDesiredObjectUIDs(ctx, kubeClient, logger, owner, scheme, desiredObjects...)
	return err
}

func pruneStaleObjects(ctx context.Context, kubeClient client.Client, logger logr.Logger, previouslyOwnedMap, desiredMap map[types.UID]client.Object) error {
	// Pruning owned objects in the cluster which should not be present after the reconciliation.
	var pruneErrs []error
	for uid, obj := range previouslyOwnedMap {
		l := logger.WithValues(
			"object_name", obj.GetName(),
			"object_kind", obj.GetObjectKind().GroupVersionKind().Kind,
		)
		if _, found := desiredMap[uid]; found {
			continue
		}

		l.Info("pruning unmanaged resource")
		err := kubeClient.Delete(ctx, obj)
		if err != nil {
			l.Error(err, "failed to delete resource")
			pruneErrs = append(pruneErrs, err)
		}
	}
	return errors.Join(pruneErrs...)
}

// enabledAcceleratedComputeByAgentConfig reports whether an AmazonCloudWatchAgent in namespace collects from the exporter behind service.
func enabledAcceleratedComputeByAgentConfig(ctx context.Context, c client.Client, log logr.Logger, namespace string, service string) (bool, error) {
	listed, err := listAmazonCloudWatchAgents(ctx, c, namespace)
	if err != nil {
		return false, err
	}
	// The exporters run on Linux nodes, so Windows agents cannot collect from them.
	var agents []v1alpha1.AmazonCloudWatchAgent
	for _, agent := range listed {
		if agent.Spec.NodeSelector[corev1.LabelOSStable] != "windows" {
			agents = append(agents, agent)
		}
	}
	// An explicit accelerated_compute_metrics: false on any agent disables the exporter for the namespace.
	for _, agent := range agents {
		if _, optedOut := acceleratedComputeByAgentJSON(agent.Spec.Config, log); optedOut {
			return false, nil
		}
	}
	for _, agent := range agents {
		enabled, _ := acceleratedComputeByAgentJSON(agent.Spec.Config, log)
		if enabled || otelConfigScrapesService(agent.Spec.OtelConfig, service, log) {
			return true, nil
		}
	}
	return false, nil
}

// acceleratedComputeByAgentJSON reports whether the agent JSON config enables accelerated compute through
// enhanced Container Insights, and whether it opts out with accelerated_compute_metrics: false.
func acceleratedComputeByAgentJSON(config string, log logr.Logger) (enabled bool, optedOut bool) {
	if config == "" {
		return false, false
	}
	conf, err := adapters.ConfigStructFromJSONString(config)
	if err != nil {
		log.Error(err, "Failed to unmarshall agent configuration")
		return false, false
	}
	if conf.Logs == nil || conf.Logs.LogMetricsCollected == nil || conf.Logs.LogMetricsCollected.Kubernetes == nil {
		return false, false
	}
	kubernetes := conf.Logs.LogMetricsCollected.Kubernetes
	// missing feature flag means it's on by default
	optedOut = strings.Contains(config, acceleratedComputeMetrics) && !kubernetes.AcceleratedComputeMetrics
	return kubernetes.EnhancedContainerInsights && !optedOut, optedOut
}

// otelConfigScrapesService reports whether a receiver used by a pipeline has a static scrape target on service.
func otelConfigScrapesService(otelConfig string, service string, log logr.Logger) bool {
	if otelConfig == "" {
		return false
	}
	conf, err := adapters.ConfigFromString(otelConfig)
	if err != nil {
		log.Error(err, "Failed to unmarshall agent OTEL configuration")
		return false
	}
	receivers, _ := conf["receivers"].(map[interface{}]interface{})
	serviceConf, _ := conf["service"].(map[interface{}]interface{})
	pipelines, _ := serviceConf["pipelines"].(map[interface{}]interface{})
	for _, p := range pipelines {
		pipeline, _ := p.(map[interface{}]interface{})
		names, _ := pipeline["receivers"].([]interface{})
		for _, name := range names {
			for _, target := range staticScrapeTargets(receivers[name]) {
				if target == service || strings.HasPrefix(target, service+":") || strings.HasPrefix(target, service+".") {
					return true
				}
			}
		}
	}
	return false
}

func staticScrapeTargets(receiver interface{}) []string {
	var targets []string
	r, _ := receiver.(map[interface{}]interface{})
	config, _ := r["config"].(map[interface{}]interface{})
	scrapeConfigs, _ := config["scrape_configs"].([]interface{})
	for _, sc := range scrapeConfigs {
		scrapeConfig, _ := sc.(map[interface{}]interface{})
		staticConfigs, _ := scrapeConfig["static_configs"].([]interface{})
		for _, st := range staticConfigs {
			staticConfig, _ := st.(map[interface{}]interface{})
			list, _ := staticConfig["targets"].([]interface{})
			for _, t := range list {
				if target, ok := t.(string); ok {
					targets = append(targets, target)
				}
			}
		}
	}
	return targets
}

var listAmazonCloudWatchAgents = func(ctx context.Context, c client.Client, namespace string) ([]v1alpha1.AmazonCloudWatchAgent, error) {
	agents := &v1alpha1.AmazonCloudWatchAgentList{}
	if err := c.List(ctx, agents, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return agents.Items, nil
}
