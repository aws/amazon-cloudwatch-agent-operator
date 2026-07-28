// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

func TestResolveCollectorNamespace(t *testing.T) {
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
		{"env empty, file whitespace only", "", ptrStr("  \n\t"), defaultCollectorNamespace},
		{"both missing falls back to default", "", nil, defaultCollectorNamespace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTELCOL_NAMESPACE", tc.env)
			orig := serviceAccountNamespacePath
			t.Cleanup(func() { serviceAccountNamespacePath = orig })
			if tc.fileBody == nil {
				serviceAccountNamespacePath = filepath.Join(t.TempDir(), "does-not-exist")
			} else {
				p := filepath.Join(t.TempDir(), "namespace")
				require.NoError(t, os.WriteFile(p, []byte(*tc.fileBody), 0o600))
				serviceAccountNamespacePath = p
			}
			assert.Equal(t, tc.want, resolveCollectorNamespace(logr.Discard()))
		})
	}
}

func ptrStr(s string) *string { return &s }

func TestNewPrometheusCRWatcherDefaultsEvaluationInterval(t *testing.T) {
	t.Setenv("OTELCOL_NAMESPACE", "eval-ns")
	cfg := allocatorconfig.Config{
		ClusterConfig: &rest.Config{Host: "https://127.0.0.1:65535"},
		PrometheusCR: allocatorconfig.PrometheusCRConfig{
			Enabled:        true,
			ScrapeInterval: model.Duration(45 * time.Second),
		},
	}
	w, err := NewPrometheusCRWatcher(logr.Discard(), cfg)
	require.NoError(t, err)
	require.NotNil(t, w.prom)
	assert.Equal(t, "eval-ns", w.prom.Namespace)
	assert.Equal(t, w.prom.Spec.ScrapeInterval, w.prom.Spec.EvaluationInterval)
	assert.Equal(t, "45s", string(w.prom.Spec.EvaluationInterval))
}
