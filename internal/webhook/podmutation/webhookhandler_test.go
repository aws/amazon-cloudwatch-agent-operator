// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package podmutation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/podmutation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/sidecar"
)

var logger = logf.Log.WithName("unit-tests")

func TestFailOnInvalidRequest(t *testing.T) {
	// we use a typical Go table-test instad of Ginkgo's DescribeTable because we need to
	// do an assertion during the declaration of the table params, which isn't supported (yet?)
	for _, tt := range []struct {
		req      admission.Request
		name     string
		expected int32
		allowed  bool
	}{
		{
			name:     "empty payload",
			req:      admission.Request{},
			expected: http.StatusBadRequest,
			allowed:  false,
		},
		{
			name: "namespace doesn't exist",
			req: func() admission.Request {
				pod := corev1.Pod{}
				encoded, err := json.Marshal(pod)
				require.NoError(t, err)

				return admission.Request{
					AdmissionRequest: admv1.AdmissionRequest{
						Namespace: "non-existing",
						Object: runtime.RawExtension{
							Raw: encoded,
						},
					},
				}
			}(),
			expected: http.StatusInternalServerError,
			allowed:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// prepare
			cfg := config.New()
			decoder := admission.NewDecoder(scheme.Scheme)
			injector := NewWebhookHandler(cfg, logger, decoder, k8sClient, []PodMutator{sidecar.NewMutator(logger, cfg, k8sClient)})

			// test
			res := injector.Handle(context.Background(), tt.req)

			// verify
			assert.Equal(t, tt.allowed, res.Allowed)
			assert.NotNil(t, res.Result)
			assert.Equal(t, tt.expected, res.Result.Code)
		})
	}
}
