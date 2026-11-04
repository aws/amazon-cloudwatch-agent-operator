// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collectornamespace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      string
		fileBody *string
		want     string
	}{
		{"env var set", "my-env-ns", nil, "my-env-ns"},
		{"env var set with whitespace", "  my-env-ns\n", nil, "my-env-ns"},
		{"env whitespace only falls through to file", "   \n", ptrStr("file-ns"), "file-ns"},
		{"env empty, file set", "", ptrStr("file-ns"), "file-ns"},
		{"env empty, file trailing newline", "", ptrStr("file-ns\n"), "file-ns"},
		{"env empty, file whitespace only", "", ptrStr("  \n\t"), Default},
		{"both missing falls back to default", "", nil, Default},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvVar, tc.env)
			setServiceAccountNamespacePath(t, tc.fileBody)
			assert.Equal(t, tc.want, Resolve(logr.Discard()))
		})
	}
}

func ptrStr(s string) *string { return &s }

// setServiceAccountNamespacePath points the service account namespace
// read at a temp file containing body, or at a missing file when body is nil.
func setServiceAccountNamespacePath(t *testing.T, body *string) {
	t.Helper()
	orig := serviceAccountNamespacePath
	t.Cleanup(func() { serviceAccountNamespacePath = orig })
	if body == nil {
		serviceAccountNamespacePath = filepath.Join(t.TempDir(), "does-not-exist")
		return
	}
	p := filepath.Join(t.TempDir(), "namespace")
	require.NoError(t, os.WriteFile(p, []byte(*body), 0o600))
	serviceAccountNamespacePath = p
}
