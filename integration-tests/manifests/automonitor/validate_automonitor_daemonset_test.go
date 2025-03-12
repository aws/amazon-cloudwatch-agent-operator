// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

func TestAutoMonitorEnabled(t *testing.T) {
	setupFunction(t, "auto-monitor-test", []string{sampleDeploymentYamlNameRelPath})
	clientSet := setupTest(t)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	updateAnnotationConfig(annotationConfig)

	if err := checkResourceAnnotations(t, clientSet, "daemonset", uniqueNamespace, daemonSetName, sampleDaemonsetYamlRelPath, startTime, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}

}
