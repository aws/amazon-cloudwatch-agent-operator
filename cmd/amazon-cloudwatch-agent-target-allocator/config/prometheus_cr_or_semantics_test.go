// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusCREnabledOrSemantics(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml bool
		args []string
		want bool
	}{
		{"yamlFalse_cliAbsent", false, nil, false},
		{"yamlFalse_cliTrue", false, []string{"--enable-prometheus-cr-watcher"}, true},
		{"yamlTrue_cliAbsent", true, nil, true},
		{"yamlTrue_cliFalse", true, []string{"--enable-prometheus-cr-watcher=false"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := getFlagSet(pflag.ContinueOnError)
			args := append([]string{"--kubeconfig-path", "./testdata/kubeconfig_test.yaml"}, tc.args...)
			require.NoError(t, fs.Parse(args))
			c := CreateDefaultConfig()
			c.PrometheusCR.Enabled = tc.yaml
			require.NoError(t, LoadFromCLI(&c, fs))
			assert.Equal(t, tc.want, c.PrometheusCR.Enabled)
		})
	}
}
