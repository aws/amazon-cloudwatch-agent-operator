// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workloadmutation contains the webhook that injects annotations into daemon-sets, deployments and stateful-sets.
package workloadmutation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
	decoder            *admission.Decoder
	annotationMutators *auto.AnnotationMutators
}

// NewWebhookHandler creates a new WorkloadWebhookHandler.
func NewWebhookHandler(decoder *admission.Decoder, annotationMutators *auto.AnnotationMutators) WebhookHandler {
	return &workloadMutationWebhook{
		decoder:            decoder,
		annotationMutators: annotationMutators,
	}
}

func (p *workloadMutationWebhook) Handle(_ context.Context, req admission.Request) admission.Response {
	var obj client.Object
	switch objectKind := req.Kind.Kind; objectKind {
	case "DaemonSet":
		obj = &appsv1.DaemonSet{}
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "StatefulSet":
		obj = &appsv1.StatefulSet{}
	default:
		return admission.Errored(http.StatusBadRequest, errors.New("failed to unmarshal request object"))
	}

	if err := p.decoder.Decode(req, obj); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	p.annotationMutators.MutateObject(obj)
	marshaledObject, err := json.Marshal(obj)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		res.Allowed = true
		return res
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObject)
}
