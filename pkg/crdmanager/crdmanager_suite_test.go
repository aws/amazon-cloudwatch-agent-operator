// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crdmanager

import (
	"fmt"
	"os"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// envClient talks to a real API server (envtest), which maintains
// metadata.generation and metadata.managedFields the way a cluster does. The
// fake client used by the unit tests does not.
var (
	envClient client.WithWatch
	envScheme = runtime.NewScheme()
)

func TestMain(m *testing.M) {
	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Printf("failed to start testEnv: %v", err)
		os.Exit(1)
	}
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, apiextensionsv1.AddToScheme} {
		if err = add(envScheme); err != nil {
			fmt.Printf("failed to register scheme: %v", err)
			os.Exit(1)
		}
	}
	envClient, err = client.NewWithWatch(cfg, client.Options{Scheme: envScheme})
	if err != nil {
		fmt.Printf("failed to set up a Kubernetes client: %v", err)
		os.Exit(1)
	}

	code := m.Run()

	if err = testEnv.Stop(); err != nil {
		fmt.Printf("failed to stop testEnv: %v", err)
		os.Exit(1)
	}
	os.Exit(code)
}
