// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

// Test_setLangEnvVars_serviceEvents verifies that the service_events sub-map from
// --auto-instrumentation-config is translated into the AUTO_INSTRUMENTATION_<LANG>_SERVICE_EVENTS_*
// env vars the instrumentation package reads, and that empty/absent values are treated as unset
// (so the SDK default applies — Service Events follows OTEL_AWS_APPLICATION_SIGNALS_ENABLED).
func Test_setLangEnvVars_serviceEvents(t *testing.T) {
	const (
		enabledVar  = "AUTO_INSTRUMENTATION_JAVA_SERVICE_EVENTS_ENABLED"
		functionVar = "AUTO_INSTRUMENTATION_JAVA_SERVICE_EVENTS_FUNCTION_INSTRUMENT_ENABLED"
		profilerVar = "AUTO_INSTRUMENTATION_JAVA_SERVICE_EVENTS_PROFILER_ENABLED"
		dynamicVar  = "AUTO_INSTRUMENTATION_JAVA_DYNAMIC_INSTRUMENTATION_ENABLED"
	)
	tests := []struct {
		name         string
		serviceEvent map[string]string
		dynamicInst  map[string]string
		wantEnabled  *string // nil = expect unset
		wantFunction *string
		wantProfiler *string
		wantDynamic  *string
	}{
		{
			name:         "absent keys leave env unset",
			serviceEvent: map[string]string{},
		},
		{
			name:         "empty enabled treated as unset",
			serviceEvent: map[string]string{"enabled": ""},
		},
		{
			name:         "enabled=false is forwarded",
			serviceEvent: map[string]string{"enabled": "false"},
			wantEnabled:  ptr("false"),
		},
		{
			name:         "profiler_enabled=false is forwarded",
			serviceEvent: map[string]string{"profiler_enabled": "false"},
			wantProfiler: ptr("false"),
		},
		{
			name:        "dynamic_instrumentation enabled=true is forwarded",
			dynamicInst: map[string]string{"enabled": "true"},
			wantDynamic: ptr("true"),
		},
		{
			name:        "empty dynamic_instrumentation enabled treated as unset",
			dynamicInst: map[string]string{"enabled": ""},
		},
		{
			name: "all toggles forwarded",
			serviceEvent: map[string]string{
				"enabled":                     "true",
				"function_instrument_enabled": "true",
				"profiler_enabled":            "false",
			},
			dynamicInst:  map[string]string{"enabled": "true"},
			wantEnabled:  ptr("true"),
			wantFunction: ptr("true"),
			wantProfiler: ptr("false"),
			wantDynamic:  ptr("true"),
		},
	}
	envVars := []string{enabledVar, functionVar, profilerVar, dynamicVar}
	unsetAll := func() {
		for _, v := range envVars {
			_ = os.Unsetenv(v)
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll()
			t.Cleanup(unsetAll)

			setLangEnvVars("JAVA", map[string]map[string]string{
				"service_events":          tt.serviceEvent,
				"dynamic_instrumentation": tt.dynamicInst,
			})

			assertEnv(t, enabledVar, tt.wantEnabled)
			assertEnv(t, functionVar, tt.wantFunction)
			assertEnv(t, profilerVar, tt.wantProfiler)
			assertEnv(t, dynamicVar, tt.wantDynamic)
		})
	}
}

func ptr(s string) *string { return &s }

func assertEnv(t *testing.T, name string, want *string) {
	t.Helper()
	got, ok := os.LookupEnv(name)
	if want == nil {
		if ok {
			t.Errorf("%s: expected unset, got %q", name, got)
		}
		return
	}
	if !ok {
		t.Errorf("%s: expected %q, but it was unset", name, *want)
		return
	}
	if got != *want {
		t.Errorf("%s: got %q, want %q", name, got, *want)
	}
}
