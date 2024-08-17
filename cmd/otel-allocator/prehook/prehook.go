// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prehook

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/otel-allocator/target"
)

const (
	relabelConfigTargetFilterName = "relabel-config"
)

type Hook interface {
	Apply(map[string]*target.Item) map[string]*target.Item
	SetConfig(map[string][]*relabel.Config)
	GetConfig() map[string][]*relabel.Config
}

type HookProvider func(log logr.Logger) Hook

var (
	registry = map[string]HookProvider{}
)

func New(name string, log logr.Logger) Hook {
	if p, ok := registry[name]; ok {
		return p(log.WithName("Prehook").WithName(name))
	}

	log.Info("Unrecognized filter strategy; filtering disabled")
	return nil
}

func Register(name string, provider HookProvider) error {
	if _, ok := registry[name]; ok {
		return errors.New("already registered")
	}
	registry[name] = provider
	return nil
}

func init() {
	err := Register(relabelConfigTargetFilterName, NewRelabelConfigTargetFilter)
	if err != nil {
		panic(err)
	}
}
