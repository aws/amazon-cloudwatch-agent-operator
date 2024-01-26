// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotation

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/daemonsetmutation"
)

type annotationDaemonSetMutator struct {
	client client.Client
	logger logr.Logger
	config config.Config
}

var _ daemonsetmutation.DaemonSetMutator = (*annotationDaemonSetMutator)(nil)

func NewMutator(logger logr.Logger, config config.Config, client client.Client) *annotationDaemonSetMutator {
	return &annotationDaemonSetMutator{
		config: config,
		logger: logger,
		client: client,
	}
}

func (a annotationDaemonSetMutator) Mutate(ctx context.Context, ds appsv1.DaemonSet) (appsv1.DaemonSet, error) {
	logger := a.logger.WithValues("namespace", ds.Namespace, "name", ds.Name)

	// check if ds and ns exists in the operator config
	isAnnotate := isAllowListed(a.config, ds)
	if !isAnnotate {
		logger.V(1).Info("annotation not present in operator config")
		// check whether ds is annotated already -- remove the annotation if that's the case
		if existsIn(ds) {
			return remove(a.config, ds)
		} else {
			return ds, nil
		}
	}

	// from this point and on, annotation is wanted
	// check whether there's annotation already -- return the same ds if that's the case.
	if existsIn(ds) {
		logger.V(1).Info("ds already has annotation in it, skipping injection")
		return ds, nil
	}

	return add(a.config, ds)
}
