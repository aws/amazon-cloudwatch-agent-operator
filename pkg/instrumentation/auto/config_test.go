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
	}
	assert.Equal(t, cfg.Java, cfg.getResources(instrumentation.TypeJava))
	assert.Equal(t, []string{"n1"}, getNamespaces(cfg.Java))
	assert.Equal(t, []string{"d1"}, getDeployments(cfg.Java))
	assert.Equal(t, cfg.Python, cfg.getResources(instrumentation.TypePython))
	assert.Equal(t, []string{"ds2"}, getDaemonSets(cfg.Python))
	assert.Equal(t, []string{"ss2"}, getStatefulSets(cfg.Python))
	assert.Equal(t, AnnotationResources{}, cfg.getResources("invalidType"))
}
