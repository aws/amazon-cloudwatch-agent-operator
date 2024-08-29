// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

func TestConfig(t *testing.T) {
	cfg := AnnotationConfig{
		Java: AnnotationResources{
			Namespaces:   []string{"n1"},
			Deployments:  []string{"d1"},
			DaemonSets:   []string{"ds1"},
			StatefulSets: []string{"ss1"},
		},
		Python: AnnotationResources{
			Namespaces:   []string{"n2"},
			Deployments:  []string{"d2"},
			DaemonSets:   []string{"ds2"},
			StatefulSets: []string{"ss2"},
		},
		DotNet: AnnotationResources{
			Namespaces:   []string{"n3"},
			Deployments:  []string{"d3"},
			DaemonSets:   []string{"ds3"},
			StatefulSets: []string{"ss3"},
		},
		NodeJS: AnnotationResources{
			Namespaces:   []string{"n3"},
			Deployments:  []string{"d3"},
			DaemonSets:   []string{"ds3"},
			StatefulSets: []string{"ss3"},
		},
	}

	assert.Equal(t, cfg.Java, cfg.getResources(instrumentation.TypeJava))
	assert.Equal(t, []string{"n1"}, getNamespaces(cfg.Java))
	assert.Equal(t, []string{"d1"}, getDeployments(cfg.Java))
	assert.Equal(t, cfg.Python, cfg.getResources(instrumentation.TypePython))
	assert.Equal(t, []string{"ds2"}, getDaemonSets(cfg.Python))
	assert.Equal(t, []string{"ss2"}, getStatefulSets(cfg.Python))
	assert.Equal(t, cfg.DotNet, cfg.getResources(instrumentation.TypeDotNet))
	assert.Equal(t, []string{"ds3"}, getDaemonSets(cfg.DotNet))
	assert.Equal(t, []string{"ss3"}, getStatefulSets(cfg.DotNet))
	assert.Equal(t, AnnotationResources{}, cfg.getResources("invalidType"))
	assert.Equal(t, cfg.NodeJS, cfg.getResources(instrumentation.TypeNodeJS))
	assert.Equal(t, []string{"ds3"}, getDaemonSets(cfg.NodeJS))
	assert.Equal(t, []string{"ss3"}, getStatefulSets(cfg.NodeJS))
}
