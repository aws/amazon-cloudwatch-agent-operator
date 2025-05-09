// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateInstrumentationAnnotator(t *testing.T) {
	// Setup
	fakeClient := fakeclient.NewClientBuilder().Build()
	ctx := context.Background()
	logger := testr.New(t)

	tests := []struct {
		name                 string
		envDisableAnnotation bool
		envDisableMonitor    bool
		autoAnnotationConfig string
		autoMonitorConfig    string
		expectNilAnnotator   bool
		expectedMonitorAll   bool
		expectedType         string
	}{
		{
			name:                 "Both annotation and monitor disabled",
			envDisableAnnotation: true,
			envDisableMonitor:    true,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   true,
			expectedMonitorAll:   false,
			expectedType:         "",
		},
		{
			name:                 "Annotation enabled, valid config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   false,
			expectedType:         "*auto.AnnotationMutators",
		},
		{
			name:                 "Annotation disabled, Monitor enabled with monitorAllServices=true",
			envDisableAnnotation: true,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   true,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Annotation disabled, Monitor enabled with monitorAllServices=false",
			envDisableAnnotation: true,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":false}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   false,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Invalid annotation config, valid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{invalid-json}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   true,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Empty annotation config, valid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   true,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Valid annotation config, invalid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{invalid-json}`,
			expectNilAnnotator:   false,
			expectedMonitorAll:   false,
			expectedType:         "*auto.AnnotationMutators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.envDisableAnnotation {
				os.Setenv("DISABLE_AUTO_ANNOTATION", "true")
			} else {
				os.Unsetenv("DISABLE_AUTO_ANNOTATION")
			}

			if tt.envDisableMonitor {
				os.Setenv("DISABLE_AUTO_MONITOR", "true")
			} else {
				os.Unsetenv("DISABLE_AUTO_MONITOR")
			}

			// Call the function
			annotator, monitorAll := createInstrumentationAnnotatorWithClientset(tt.autoMonitorConfig, tt.autoAnnotationConfig, ctx, fake.NewSimpleClientset(), fakeClient, fakeClient, logger)

			// Check results
			if tt.expectNilAnnotator {
				assert.Nil(t, annotator, "Expected nil annotator")
			} else {
				assert.NotNil(t, annotator, "Expected non-nil annotator")

				// Check type using type assertion
				actualType := fmt.Sprintf("%T", annotator)
				assert.Equal(t, tt.expectedType, actualType, "Unexpected annotator type")

				// Specific type assertions
				switch tt.expectedType {
				case "*auto.AnnotationMutators":
					_, ok := annotator.(*AnnotationMutators)
					assert.True(t, ok, "Expected annotator to be of type *AnnotationMutators")
				case "*auto.Monitor":
					monitor, ok := annotator.(*Monitor)
					assert.True(t, ok, "Expected annotator to be of type *Monitor")
					if tt.expectedMonitorAll {
						assert.True(t, monitor.config.MonitorAllServices, "Expected MonitorAllServices to be true")
					} else {
						assert.False(t, monitor.config.MonitorAllServices, "Expected MonitorAllServices to be false")
					}
				}
			}

			assert.Equal(t, tt.expectedMonitorAll, monitorAll, "Unexpected monitorAll value")
		})
	}
}
