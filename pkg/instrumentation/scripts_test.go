// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestEmbeddedScripts_TreatDollarOneAsLiteral is the runtime exploit-replay
// for the P431312609 hardening: it invokes each embedded init-container
// script via /bin/sh -c <script> -- <arg> with a malicious arg that *would*
// touch a sentinel file if the script ever spliced $1 into a shell-parsed
// string. We assert the sentinel never appears, regardless of any cp/sed
// failures the script may hit on the fake env we provide.
func TestEmbeddedScripts_TreatDollarOneAsLiteral(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell")
	}

	tests := []struct {
		name   string
		script string
	}{
		{"apacheHttpdAgentScript", apacheHttpdAgentScript},
		{"nginxCloneScript", nginxCloneScript},
		{"nginxAgentScript", nginxAgentScript},
	}

	// One sandbox per test invocation; share a single marker path so each
	// subtest sees a clean slate (defensive cleanup defends against any
	// stray prior touch).
	sandbox := t.TempDir()
	marker := filepath.Join(sandbox, "PWN")

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Defensive cleanup before AND after, so a prior subtest's
			// regression doesn't leak forward.
			_ = os.Remove(marker)
			defer func() { _ = os.Remove(marker) }()

			// Malicious $1: if the script were to do `eval "...$1..."`
			// or `cmd ...$1...` without quoting, this would touch <marker>.
			arg := "; touch " + marker + " #"

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", tc.script, "--", arg)
			// Provide enough env that the scripts can reference their
			// expected variables without panicking. PATH is required so
			// `cp`, `sed`, `cat`, `printf` resolve.
			cmd.Env = []string{
				"OTEL_APACHE_AGENT_CONF=foo",
				"APACHE_SERVICE_INSTANCE_ID=test",
				"OTEL_NGINX_AGENT_CONF=foo",
				"OTEL_NGINX_I13N_SCRIPT=foo",
				"OTEL_NGINX_SERVICE_INSTANCE_ID=test",
				"PATH=/usr/bin:/bin",
			}

			var combined bytes.Buffer
			cmd.Stdout = &combined
			cmd.Stderr = &combined

			// Script is expected to fail (cp from nonexistent /opt/opentelemetry,
			// missing version.txt, etc.). We don't care — we only care about
			// the marker file. Ignoring exit status is intentional.
			_ = cmd.Run()

			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("marker %q exists -> $1 was spliced as code (regression of P431312609); script combined output:\n%s",
					marker, combined.String())
			}
		})
	}
}
