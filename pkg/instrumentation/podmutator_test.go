// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testScheme = scheme.Scheme
)

func TestGetInstrumentationInstanceFromNameSpaceDefault(t *testing.T) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-namespace",
		},
	}
	if err := v1alpha1.AddToScheme(testScheme); err != nil {
		fmt.Printf("failed to register scheme: %v", err)
		os.Exit(1)
	}
	podMutator := instPodMutator{
		Client: fake.NewClientBuilder().Build(),
		Logger: logr.Logger{},
	}
	instrumentation, err := podMutator.selectInstrumentationInstanceFromNamespace(context.Background(), namespace)

	assert.Nil(t, err)
	defaultInst, _ := getDefaultInstrumentation()
	assert.Equal(t, defaultInst, instrumentation)
}
