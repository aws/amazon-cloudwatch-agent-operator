// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package autodetect is for auto-detecting traits from the environment (platform, APIs, ...).
package autodetect

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/autodetect/openshift"
)

var _ AutoDetect = (*autoDetect)(nil)

// AutoDetect provides an assortment of routines that auto-detect traits based on the runtime.
type AutoDetect interface {
	OpenShiftRoutesAvailability() (openshift.RoutesAvailability, error)
}

type autoDetect struct {
	dcl discovery.DiscoveryInterface
}

// New creates a new auto-detection worker, using the given client when talking to the current cluster.
func New(restConfig *rest.Config) (AutoDetect, error) {
	dcl, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		// it's pretty much impossible to get into this problem, as most of the
		// code branches from the previous call just won't fail at all,
		// but let's handle this error anyway...
		return nil, err
	}

	return &autoDetect{
		dcl: dcl,
	}, nil
}

// OpenShiftRoutesAvailability checks if OpenShift Route are available.
func (a *autoDetect) OpenShiftRoutesAvailability() (openshift.RoutesAvailability, error) {
	apiList, err := a.dcl.ServerGroups()
	if err != nil {
		return openshift.RoutesNotAvailable, err
	}

	apiGroups := apiList.Groups
	for i := 0; i < len(apiGroups); i++ {
		if apiGroups[i].Name == "route.openshift.io" {
			return openshift.RoutesAvailable, nil
		}
	}

	return openshift.RoutesNotAvailable, nil
}
