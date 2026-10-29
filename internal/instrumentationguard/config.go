// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package instrumentationguard reports auto-instrumented pods that break shortly after they start
// AND whose crash output holds auto-instrumentation responsible, so a customer whose application
// was broken by injection finds out from the cluster instead of from a crash loop nobody can
// explain.
//
// It is DETECT-ONLY. It is one controller: it watches auto-instrumented pods, decides that a pod
// is broken (health.go) and whether that is the operator's doing (attribution.go), and on both
// counts emits a Warning Event on the owning workload naming the pod, the injected languages and
// images, and what to change to stop the injection. It never writes to a customer's workload, and
// it never deletes a pod.
//
// It stops there deliberately. Attribution is pattern matching on crash output, and crash output
// that is innocent can be indistinguishable from crash output that is guilty - see the known
// limitations in README.md - so the decision to give up telemetry belongs to the customer, who can
// read the pod, and not to the operator, which can only read the text.
package instrumentationguard

import (
	"fmt"
	"time"
)

// Mode selects whether the instrumentation guard runs. There is no mode that writes to a
// customer's workload: the guard is detect-only, so ModeDryRun is as far as it goes and is the
// only mode in which it does anything at all.
type Mode string

const (
	// ModeOff disables the guard entirely: main.go registers no controller for it AND it leaves
	// instrumentation.NewStampMutator out of the pod mutator chain, so an injected pod gets
	// neither the auto-instrumented label nor the terminationMessagePolicy the guard reads crash
	// output through. Nothing of the guard runs and nothing of it is visible on a pod.
	ModeOff Mode = "off"
	// ModeDryRun watches, attributes and emits Events. It writes nothing. It is the guard's only
	// active mode.
	ModeDryRun Mode = "dry-run"
)

// ParseMode converts a flag value into a Mode, rejecting anything else.
func ParseMode(s string) (Mode, error) {
	switch mode := Mode(s); mode {
	case ModeOff, ModeDryRun:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid instrumentation guard mode %q: must be one of %q, %q", s, ModeOff, ModeDryRun)
	}
}

// Config holds the guard's tunables.
type Config struct {
	// Mode selects whether the guard runs at all.
	Mode Mode
	// RestartThreshold is the container restart count at which a pod counts as broken. It is an
	// int32 to compare directly with corev1.ContainerStatus.RestartCount.
	RestartThreshold int32
	// Window is how long after a pod starts the guard attributes a failure to auto-instrumentation.
	Window time.Duration
	// ImagePullPatience is how long an auto-instrumentation init container may sit unable to pull
	// its image before the guard calls the pod broken. A failed copy is deterministic and counts
	// immediately, but a failed pull is often a transient registry problem, so it gets more
	// patience than any other trigger.
	ImagePullPatience time.Duration
	// RequeueInterval is how often the guard re-checks a pod or workload it is still waiting on.
	RequeueInterval time.Duration
}

// DefaultConfig returns the guard's defaults: dry-run - the only mode that does anything - 3
// restarts, a 10 minute window, and 5 minutes of patience for an image pull.
func DefaultConfig() Config {
	return Config{
		Mode:              ModeDryRun,
		RestartThreshold:  3,
		Window:            10 * time.Minute,
		ImagePullPatience: 5 * time.Minute,
		RequeueInterval:   30 * time.Second,
	}
}

// Validate reports why the configuration is unusable, if it is.
func (c Config) Validate() error {
	switch c.Mode {
	case ModeOff, ModeDryRun:
	default:
		return fmt.Errorf("invalid instrumentation guard mode %q: must be one of %q, %q", c.Mode, ModeOff, ModeDryRun)
	}
	if c.RestartThreshold < 1 {
		return fmt.Errorf("instrumentation guard restart threshold must be at least 1, got %d", c.RestartThreshold)
	}
	for _, d := range []struct {
		name  string
		value time.Duration
	}{
		{"window", c.Window},
		{"image pull patience", c.ImagePullPatience},
		{"requeue interval", c.RequeueInterval},
	} {
		if d.value <= 0 {
			return fmt.Errorf("instrumentation guard %s must be positive, got %s", d.name, d.value)
		}
	}
	if c.ImagePullPatience > c.Window {
		// A patience longer than the attribution window could never fire: the pod leaves the
		// window before the guard runs out of patience.
		return fmt.Errorf("instrumentation guard image pull patience (%s) must not be greater than the window (%s)", c.ImagePullPatience, c.Window)
	}
	return nil
}
