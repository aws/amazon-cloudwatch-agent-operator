// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package daemonsetmutation contains the webhook that injects annotations into daemon-sets.
package daemonsetmutation

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

// +kubebuilder:webhook:path=/mutate-v1-daemonset,mutating=true,failurePolicy=ignore,groups="apps",resources=daemonsets,verbs=create;update,versions=v1,name=mdaemonset.kb.io,sideEffects=none,admissionReviewVersions=v1
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch
// +kubebuilder:rbac:groups="apps",resources=daemonsets,verbs=get;list;watch

var _ WebhookHandler = (*daemonSetMutationWebhook)(nil)

// WebhookHandler is a webhook handler that analyzes new daemon-sets and injects appropriate annotations into it.
type WebhookHandler interface {
	admission.Handler
}

// the implementation.
type daemonSetMutationWebhook struct {
	client            client.Client
	decoder           *admission.Decoder
	logger            logr.Logger
	daemonSetMutators []DaemonSetMutator
	config            config.Config
}

// DaemonSetMutator mutates a daemon-set.
type DaemonSetMutator interface {
	Mutate(ctx context.Context, ds appsv1.DaemonSet) (appsv1.DaemonSet, error)
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(cfg config.Config, logger logr.Logger, decoder *admission.Decoder, cl client.Client, daemonSetMutators []DaemonSetMutator) WebhookHandler {
	return &daemonSetMutationWebhook{
		config:            cfg,
		decoder:           decoder,
		logger:            logger,
		client:            cl,
		daemonSetMutators: daemonSetMutators,
	}
}

func (p *daemonSetMutationWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	ds := appsv1.DaemonSet{}
	err := p.decoder.Decode(req, &ds)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// we use the req.Namespace here because the pod might have not been created yet
	ns := corev1.Namespace{}
	err = p.client.Get(ctx, types.NamespacedName{Name: req.Namespace, Namespace: ""}, &ns)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		// By default, admission.Errored sets Allowed to false which blocks pod creation even though the failurePolicy=ignore.
		// Allowed set to true makes sure failure does not block pod creation in case of an error.
		// Using the http.StatusInternalServerError creates a k8s event associated with the replica set.
		// The admission.Allowed("").WithWarnings(err.Error()) or http.StatusBadRequest does not
		// create any event. Additionally, an event/log cannot be created explicitly because the pod name is not known.
		res.Allowed = true
		return res
	}

	for _, m := range p.daemonSetMutators {
		ds, err = m.Mutate(ctx, ds)
		if err != nil {
			res := admission.Errored(http.StatusInternalServerError, err)
			res.Allowed = true
			return res
		}
	}

	marshaledPod, err := json.Marshal(ds)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		res.Allowed = true
		return res
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
