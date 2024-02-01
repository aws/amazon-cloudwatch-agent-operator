// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workloadmutation contains the webhook that injects annotations into daemon-sets, deployments and stateful-sets.
package workloadmutation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

// +kubebuilder:webhook:path=/mutate-v1-workload,mutating=true,failurePolicy=ignore,groups="apps",resources=daemonsets;deployments;statefulsets,verbs=create;update,versions=v1,name=mworkload.kb.io,sideEffects=none,admissionReviewVersions=v1
// +kubebuilder:rbac:groups="apps",resources=daemonsets;deployments;statefulsets,verbs=get;list;watch

var _ WebhookHandler = (*workloadMutationWebhook)(nil)

// WebhookHandler is a webhook handler that analyzes new daemon-sets and injects appropriate annotations into it.
type WebhookHandler interface {
	admission.Handler
}

// the implementation.
type workloadMutationWebhook struct {
	client            client.Client
	decoder           *admission.Decoder
	logger            logr.Logger
	config            config.Config
	annotationMutator *auto.AnnotationMutators
}

// NewWebhookHandler creates a new WorkloadWebhookHandler.
func NewWebhookHandler(cfg config.Config, logger logr.Logger, decoder *admission.Decoder, cl client.Client, annotationMutation *auto.AnnotationMutators) WebhookHandler {
	return &workloadMutationWebhook{
		config:            cfg,
		decoder:           decoder,
		logger:            logger,
		client:            cl,
		annotationMutator: annotationMutation,
	}
}

func (p *workloadMutationWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	if p.annotationMutator == nil {
		// By default, admission.Errored sets Allowed to false which blocks workload creation even though the failurePolicy=ignore.
		// Allowed set to true makes sure failure does not block workload creation in case of an error.
		// Returning http.StatusBadRequest does not create any event.
		res := admission.Errored(http.StatusBadRequest, errors.New("failed to unmarshal annotation config"))
		res.Allowed = true
		return res
	}

	var err error
	var marshaledObject []byte
	var object runtime.Object
	switch objectKind := req.Kind.Kind; objectKind {
	case "DaemonSet":
		ds := appsv1.DaemonSet{}
		err = p.decoder.Decode(req, &ds)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		object = &ds
	case "Deployment":
		d := appsv1.Deployment{}
		err = p.decoder.Decode(req, &d)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		object = &d
	case "StatefulSet":
		ss := appsv1.StatefulSet{}
		err = p.decoder.Decode(req, &ss)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		object = &ss
	default:
		return admission.Errored(http.StatusBadRequest, errors.New("failed to unmarshal request object"))
	}

	p.annotationMutator.Mutate(object)
	marshaledObject, err = json.Marshal(object)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		res.Allowed = true
		return res
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObject)
}
