// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacemutation

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

// +kubebuilder:webhook:path=/mutate-v1-namespace,mutating=true,failurePolicy=ignore,groups="",resources=namespaces,verbs=create;update,versions=v1,name=mnamespace.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ admission.Handler = (*handler)(nil)

type handler struct {
	decoder                  admission.Decoder
	instrumentationAnnotator auto.InstrumentationAnnotator
}

func NewWebhookHandler(decoder admission.Decoder, instrumentationAnnotator auto.InstrumentationAnnotator) admission.Handler {
	return &handler{
		decoder:                  decoder,
		instrumentationAnnotator: instrumentationAnnotator,
	}
}

func (h *handler) Handle(_ context.Context, req admission.Request) admission.Response {
	namespace := &corev1.Namespace{}
	err := h.decoder.Decode(req, namespace)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// do not need to pass in oldObj because it's only used to check for workload pod template diff
	h.instrumentationAnnotator.MutateObject(nil, namespace)

	marshaledNamespace, err := json.Marshal(namespace)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		res.Allowed = true
		return res
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledNamespace)
}
