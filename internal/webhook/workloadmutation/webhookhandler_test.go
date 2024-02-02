// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workloadmutation

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

var (
	k8sClient client.Client
	logger    = logf.Log.WithName("unit-tests")
)

func TestInvalidRequest(t *testing.T) {
	for _, tt := range []struct {
		req                  admission.Request
		autoAnnotationConfig auto.AnnotationConfig
		name                 string
		expected             int32
		allowed              bool
	}{
		{
			name:     "invalid payload",
			req:      admission.Request{},
			expected: http.StatusBadRequest,
			allowed:  false,
			autoAnnotationConfig: auto.AnnotationConfig{
				Java: auto.AnnotationResources{
					Namespaces: []string{"keep-auto-java"},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// prepare
			cfg := config.New()
			decoder := admission.NewDecoder(scheme.Scheme)
			mutators := auto.NewAnnotationMutators(
				k8sClient,
				k8sClient,
				logr.Logger{},
				tt.autoAnnotationConfig,
				instrumentation.NewTypeSet(instrumentation.TypeJava),
			)
			injector := NewWebhookHandler(cfg, logger, decoder, k8sClient, mutators)

			// test
			res := injector.Handle(context.Background(), tt.req)

			// verify
			assert.Equal(t, tt.allowed, res.Allowed)
			assert.NotNil(t, res.AdmissionResponse.Result)
			assert.Equal(t, tt.expected, res.AdmissionResponse.Result.Code)
		})
	}
}
