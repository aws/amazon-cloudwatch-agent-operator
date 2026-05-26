// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// TestInstrumentationCRD_RegexMarkerPreserved is a regression-guard for the
// P431312609 hardening: the kubebuilder marker on
// {ApacheHttpd.ConfigPath, Nginx.ConfigFile} that restricts the value to
// `^[A-Za-z0-9._/-]*$` (and bounds it to 256 chars) MUST flow through
// `make manifests` into the generated CRD YAML. If someone accidentally
// drops the marker on the Go type, the regenerated CRD will silently lose
// its admission-time defense and this test will fail.
func TestInstrumentationCRD_RegexMarkerPreserved(t *testing.T) {
	// This test lives in apis/v1alpha1/, so the CRD is two dirs up.
	crdPath := filepath.Join("..", "..", "config", "crd", "bases", "cloudwatch.aws.amazon.com_instrumentations.yaml")

	data, err := os.ReadFile(crdPath)
	if err != nil {
		// We deliberately do NOT silently pass — but we DO skip with an
		// explanatory message if the path resolution is impossible (e.g.
		// running from a vendored copy with no config/ tree).
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("CRD file not found at %q (test must run from repo with config/crd/bases/ available): %v", crdPath, err)
		}
		t.Fatalf("failed to read CRD file at %q: %v", crdPath, err)
	}

	var crd map[string]any
	require.NoError(t, yaml.Unmarshal(data, &crd), "yaml unmarshal of CRD failed")

	specRoot, ok := crd["spec"].(map[string]any)
	require.True(t, ok, "crd.spec missing or not a map")

	versions, ok := specRoot["versions"].([]any)
	require.True(t, ok, "crd.spec.versions missing or not a list")
	require.NotEmpty(t, versions, "crd.spec.versions is empty")

	v0, ok := versions[0].(map[string]any)
	require.True(t, ok, "crd.spec.versions[0] not a map")

	schema, ok := v0["schema"].(map[string]any)
	require.True(t, ok, "versions[0].schema missing")
	openAPI, ok := schema["openAPIV3Schema"].(map[string]any)
	require.True(t, ok, "openAPIV3Schema missing")
	rootProps, ok := openAPI["properties"].(map[string]any)
	require.True(t, ok, "openAPIV3Schema.properties missing")
	specSchema, ok := rootProps["spec"].(map[string]any)
	require.True(t, ok, "properties.spec missing")
	specProps, ok := specSchema["properties"].(map[string]any)
	require.True(t, ok, "spec.properties missing")

	cases := []struct {
		parent string
		field  string
	}{
		{"apacheHttpd", "configPath"},
		{"nginx", "configFile"},
	}
	for _, tc := range cases {
		t.Run(tc.parent+"."+tc.field, func(t *testing.T) {
			parentNode, ok := specProps[tc.parent].(map[string]any)
			require.Truef(t, ok, "spec.properties.%s missing", tc.parent)
			parentProps, ok := parentNode["properties"].(map[string]any)
			require.Truef(t, ok, "spec.properties.%s.properties missing", tc.parent)
			fieldNode, ok := parentProps[tc.field].(map[string]any)
			require.Truef(t, ok, "spec.properties.%s.properties.%s missing", tc.parent, tc.field)

			require.Equal(t, "^[A-Za-z0-9._/-]*$", fieldNode["pattern"],
				"%s.%s pattern marker missing/changed", tc.parent, tc.field)
			// sigs.k8s.io/yaml routes through JSON, so numeric literals
			// arrive as float64 — use EqualValues for cross-type equality.
			require.EqualValues(t, 256, fieldNode["maxLength"],
				"%s.%s maxLength marker missing/changed", tc.parent, tc.field)
		})
	}
}
