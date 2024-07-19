// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
)

const (
	acceleratedComputeMetrics = "accelerated_compute_metrics"
	amazonCloudWatchNamespace = "amazon-cloudwatch"
	amazonCloudWatchAgentName = "cloudwatch-agent"
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

// getList queries the Kubernetes API to list the requested resource, setting the list l of type T.
func getList[T client.Object](ctx context.Context, cl client.Client, l T, options ...client.ListOption) (map[types.UID]client.Object, error) {
	ownedObjects := map[types.UID]client.Object{}
	list := &unstructured.UnstructuredList{}
	gvk, err := apiutil.GVKForObject(l, cl.Scheme())
	if err != nil {
		return nil, err
	}
	list.SetGroupVersionKind(gvk)
	err = cl.List(ctx, list, options...)
	if err != nil {
		return ownedObjects, fmt.Errorf("error listing %T: %w", l, err)
	}
	for i := range list.Items {
		ownedObjects[list.Items[i].GetUID()] = &list.Items[i]
	}
	return ownedObjects, nil
}

// reconcileDesiredObjects runs the reconcile process using the mutateFn over the given list of objects.
func reconcileDesiredObjects(ctx context.Context, kubeClient client.Client, logger logr.Logger, owner metav1.Object, scheme *runtime.Scheme, desiredObjects ...client.Object) error {
	var errs []error
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
		mutateFn := manifests.MutateFuncFor(existing, desired)
		var op controllerutil.OperationResult
		crudErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			result, createOrUpdateErr := ctrl.CreateOrUpdate(ctx, kubeClient, existing, mutateFn)
			op = result
			return createOrUpdateErr
		})
		if crudErr != nil && errors.Is(crudErr, manifests.ImmutableChangeErr) {
			l.Error(crudErr, "detected immutable field change, trying to delete, new object will be created on next reconcile", "existing", existing.GetName())
			delErr := kubeClient.Delete(ctx, existing)
			if delErr != nil {
				return delErr
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
		return fmt.Errorf("failed to create objects for %s: %w", owner.GetName(), errors.Join(errs...))
	}
	return nil
}

func enabledAcceleratedComputeByAgentConfig(ctx context.Context, c client.Client, log logr.Logger) bool {
	agentResource := getAmazonCloudWatchAgentResource(ctx, c)
	// missing feature flag means it's on by default
	featureConfigExists := strings.Contains(agentResource.Spec.Config, acceleratedComputeMetrics)
	conf, err := adapters.ConfigStructFromJSONString(agentResource.Spec.Config)
	if err != nil {
		log.Error(err, "Failed to unmarshall agent configuration")
		return false
	}

	if conf.Logs != nil && conf.Logs.LogMetricsCollected != nil && conf.Logs.LogMetricsCollected.Kubernetes != nil {
		if conf.Logs.LogMetricsCollected.Kubernetes.EnhancedContainerInsights {
			return !featureConfigExists || conf.Logs.LogMetricsCollected.Kubernetes.AcceleratedComputeMetrics
		} else {
			// enhanced container insights is disabled
			return false
		}
	}
	return false
}

var getAmazonCloudWatchAgentResource = func(ctx context.Context, c client.Client) v1alpha1.AmazonCloudWatchAgent {
	cr := &v1alpha1.AmazonCloudWatchAgent{}

	_ = c.Get(ctx, client.ObjectKey{
		Namespace: amazonCloudWatchNamespace,
		Name:      amazonCloudWatchAgentName,
	}, cr)

	return *cr
}

func deleteObjects(ctx context.Context, kubeClient client.Client, logger logr.Logger, objects map[types.UID]client.Object) error {
	// Pruning owned objects in the cluster which are not should not be present after the reconciliation.
	pruneErrs := []error{}
	for _, obj := range objects {
		l := logger.WithValues(
			"object_name", obj.GetName(),
			"object_kind", obj.GetObjectKind().GroupVersionKind(),
		)

		l.Info("pruning unmanaged resource")
		err := kubeClient.Delete(ctx, obj)
		if err != nil {
			l.Error(err, "failed to delete resource")
			pruneErrs = append(pruneErrs, err)
		}
	}
	return errors.Join(pruneErrs...)
}
