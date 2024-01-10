// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/targetallocator"
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
