// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

var testTolerationValues = []v1.Toleration{
	{
		Key:    "hii",
		Value:  "greeting",
		Effect: "NoSchedule",
	},
}

var testTopologySpreadConstraintValue = []v1.TopologySpreadConstraint{
	{
		MaxSkew:           1,
		TopologyKey:       "kubernetes.io/hostname",
		WhenUnsatisfiable: "DoNotSchedule",
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"foo": "bar",
			},
		},
	},
}

var testAffinityValue = &v1.Affinity{
	NodeAffinity: &v1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{
				{
					MatchExpressions: []v1.NodeSelectorRequirement{
						{
							Key:      "node",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"test-node"},
						},
					},
				},
			},
		},
	},
}

var runAsUser int64 = 1000
var runAsGroup int64 = 1000

var testSecurityContextValue = &v1.PodSecurityContext{
	RunAsUser:  &runAsUser,
	RunAsGroup: &runAsGroup,
}

func TestDeploymentSecurityContext(t *testing.T) {
	// Test default
	otelcol1 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		OtelCol: otelcol1,
		Config:  cfg,
		Log:     logger,
	}
	d1, err := Deployment(params1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, d1.Spec.Template.Spec.SecurityContext)

	// Test SecurityContext
	otelcol2 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-securitycontext",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				SecurityContext: testSecurityContextValue,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		OtelCol: otelcol2,
		Config:  cfg,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, *testSecurityContextValue, *d2.Spec.Template.Spec.SecurityContext)
}

func TestDeploymentNewDefault(t *testing.T) {
	// prepare
	otelcol := collectorInstance()
	cfg := config.New()

	params := manifests.Params{
		OtelCol: otelcol,
		Config:  cfg,
		Log:     logger,
	}

	// test
	d, err := Deployment(params)

	assert.NoError(t, err)

	// verify
	assert.Equal(t, "my-instance-target-allocator", d.GetName())
	assert.Equal(t, "my-instance-target-allocator", d.GetLabels()["app.kubernetes.io/name"])

	assert.Len(t, d.Spec.Template.Spec.Containers, 1)

	// should only have the ConfigMap hash annotation
	assert.Contains(t, d.Spec.Template.Annotations, configMapHashAnnotationKey)
	assert.Len(t, d.Spec.Template.Annotations, 1)

	// the pod selector should match the pod spec's labels
	assert.Equal(t, d.Spec.Template.Labels, d.Spec.Selector.MatchLabels)
}

func TestDeploymentPodAnnotations(t *testing.T) {
	// prepare
	testPodAnnotationValues := map[string]string{"annotation-key": "annotation-value"}
	otelcol := collectorInstance()
	otelcol.Spec.PodAnnotations = testPodAnnotationValues
	cfg := config.New()

	params := manifests.Params{
		OtelCol: otelcol,
		Config:  cfg,
		Log:     logger,
	}

	// test
	ds, err := Deployment(params)
	assert.NoError(t, err)
	// verify
	assert.Equal(t, "my-instance-target-allocator", ds.Name)
	assert.Subset(t, ds.Spec.Template.Annotations, testPodAnnotationValues)
}

func collectorInstance() v1alpha1.AmazonCloudWatchAgent {
	configYAML, err := os.ReadFile("testdata/test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	return v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "default",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image:            "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator:0.47.0",
			PrometheusConfig: string(configYAML),
		},
	}
}

func TestDeploymentNodeSelector(t *testing.T) {
	// Test default
	otelcol1 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		OtelCol: otelcol1,
		Config:  cfg,
		Log:     logger,
	}
	d1, err := Deployment(params1)
	assert.NoError(t, err)
	assert.Empty(t, d1.Spec.Template.Spec.NodeSelector)

	// Test nodeSelector
	otelcol2 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-nodeselector",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				NodeSelector: map[string]string{
					"node-key": "node-value",
				},
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		OtelCol: otelcol2,
		Config:  cfg,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"node-key": "node-value"}, d2.Spec.Template.Spec.NodeSelector)
}
func TestDeploymentAffinity(t *testing.T) {
	// Test default
	otelcol1 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		OtelCol: otelcol1,
		Config:  cfg,
		Log:     logger,
	}
	d1, err := Deployment(params1)
	assert.NoError(t, err)
	assert.Empty(t, d1.Spec.Template.Spec.Affinity)

	// Test affinity
	otelcol2 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-affinity",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Affinity: testAffinityValue,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		OtelCol: otelcol2,
		Config:  cfg,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	assert.NoError(t, err)
	assert.Equal(t, *testAffinityValue, *d2.Spec.Template.Spec.Affinity)
}

func TestDeploymentTolerations(t *testing.T) {
	// Test default
	otelcol1 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()
	params1 := manifests.Params{
		OtelCol: otelcol1,
		Config:  cfg,
		Log:     logger,
	}
	d1, err := Deployment(params1)
	assert.NoError(t, err)
	assert.Equal(t, "my-instance-target-allocator", d1.Name)
	assert.Empty(t, d1.Spec.Template.Spec.Tolerations)

	// Test Tolerations
	otelcol2 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-toleration",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Tolerations: testTolerationValues,
			},
		},
	}

	params2 := manifests.Params{
		OtelCol: otelcol2,
		Config:  cfg,
		Log:     logger,
	}
	d2, err := Deployment(params2)
	assert.NoError(t, err)
	assert.Equal(t, "my-instance-toleration-target-allocator", d2.Name)
	assert.NotNil(t, d2.Spec.Template.Spec.Tolerations)
	assert.NotEmpty(t, d2.Spec.Template.Spec.Tolerations)
	assert.Equal(t, testTolerationValues, d2.Spec.Template.Spec.Tolerations)
}

func TestDeploymentTopologySpreadConstraints(t *testing.T) {
	// Test default
	otelcol1 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		OtelCol: otelcol1,
		Config:  cfg,
		Log:     logger,
	}
	d1, err := Deployment(params1)
	assert.NoError(t, err)
	assert.Equal(t, "my-instance-target-allocator", d1.Name)
	assert.Empty(t, d1.Spec.Template.Spec.TopologySpreadConstraints)

	// Test TopologySpreadConstraints
	otelcol2 := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-topologyspreadconstraint",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				TopologySpreadConstraints: testTopologySpreadConstraintValue,
			},
		},
	}

	cfg = config.New()
	params2 := manifests.Params{
		OtelCol: otelcol2,
		Config:  cfg,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	assert.NoError(t, err)
	assert.Equal(t, "my-instance-topologyspreadconstraint-target-allocator", d2.Name)
	assert.NotNil(t, d2.Spec.Template.Spec.TopologySpreadConstraints)
	assert.NotEmpty(t, d2.Spec.Template.Spec.TopologySpreadConstraints)
	assert.Equal(t, testTopologySpreadConstraintValue, d2.Spec.Template.Spec.TopologySpreadConstraints)
}
