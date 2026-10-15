// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

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
