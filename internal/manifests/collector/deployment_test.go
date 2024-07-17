// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build ignore_test

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
)

var testTolerationValues = []v1.Toleration{
	{
		Key:    "hii",
		Value:  "greeting",
		Effect: "NoSchedule",
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

func TestDeploymentNewDefault(t *testing.T) {
	// prepare
	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-namespace",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				Tolerations: testTolerationValues,
			},
		},
	}
	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	// test
	d, err := Deployment(params)
	require.NoError(t, err)

	// verify
	assert.Equal(t, "my-instance", d.Name)
	assert.Equal(t, "my-instance", d.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "true", d.Annotations["prometheus.io/scrape"])
	assert.Equal(t, "8888", d.Annotations["prometheus.io/port"])
	assert.Equal(t, "/metrics", d.Annotations["prometheus.io/path"])
	assert.Equal(t, testTolerationValues, d.Spec.Template.Spec.Tolerations)

	assert.Len(t, d.Spec.Template.Spec.Containers, 1)

	// verify sha256 podAnnotation
	expectedAnnotations := map[string]string{
		"amazon-cloudwatch-agent-operator-config/sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"prometheus.io/path":                             "/metrics",
		"prometheus.io/port":                             "8888",
		"prometheus.io/scrape":                           "true",
	}
	assert.Equal(t, expectedAnnotations, d.Spec.Template.Annotations)

	expectedLabels := map[string]string{
		"app.kubernetes.io/component":  "amazon-cloudwatch-agent",
		"app.kubernetes.io/instance":   "my-namespace.my-instance",
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/name":       "my-instance",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "latest",
	}
	assert.Equal(t, expectedLabels, d.Spec.Template.Labels)

	expectedSelectorLabels := map[string]string{
		"app.kubernetes.io/component":  "amazon-cloudwatch-agent",
		"app.kubernetes.io/instance":   "my-namespace.my-instance",
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
	}
	assert.Equal(t, expectedSelectorLabels, d.Spec.Selector.MatchLabels)

	// the pod selector must be contained within pod spec's labels
	for k, v := range d.Spec.Selector.MatchLabels {
		assert.Equal(t, v, d.Spec.Template.Labels[k])
	}
}

func TestDeploymentPodAnnotations(t *testing.T) {
	// prepare
	testPodAnnotationValues := map[string]string{"annotation-key": "annotation-value"}
	otelcol := v1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1beta1.OpenTelemetryCollectorSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				PodAnnotations: testPodAnnotationValues,
			},
		},
	}
	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	// test
	d, err := Deployment(params)
	require.NoError(t, err)

	// Add sha256 podAnnotation
	testPodAnnotationValues["amazon-cloudwatch-agent-operator-config/sha256"] = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	expectedPodAnnotationValues := map[string]string{
		"annotation-key": "annotation-value",
		"amazon-cloudwatch-agent-operator-config/sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"prometheus.io/path":                             "/metrics",
		"prometheus.io/port":                             "8888",
		"prometheus.io/scrape":                           "true",
	}

	// verify
	assert.Len(t, d.Spec.Template.Annotations, 5)
	assert.Equal(t, "my-instance", d.Name)
	assert.Equal(t, expectedPodAnnotationValues, d.Spec.Template.Annotations)
}

func TestDeploymenttPodSecurityContext(t *testing.T) {
	runAsNonRoot := true
	runAsUser := int64(1337)
	runasGroup := int64(1338)

	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				PodSecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: &runAsNonRoot,
					RunAsUser:    &runAsUser,
					RunAsGroup:   &runasGroup,
				},
			},
		},
	}

	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	d, err := Deployment(params)
	require.NoError(t, err)

	assert.Equal(t, &runAsNonRoot, d.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
	assert.Equal(t, &runAsUser, d.Spec.Template.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, &runasGroup, d.Spec.Template.Spec.SecurityContext.RunAsGroup)
}

func TestDeploymentUpdateStrategy(t *testing.T) {
	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			DeploymentUpdateStrategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(1)},
				},
			},
		},
	}

	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	d, err := Deployment(params)
	require.NoError(t, err)

	assert.Equal(t, "RollingUpdate", string(d.Spec.Strategy.Type))
	assert.Equal(t, 1, d.Spec.Strategy.RollingUpdate.MaxSurge.IntValue())
	assert.Equal(t, 1, d.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue())
}

func TestDeploymentHostNetwork(t *testing.T) {
	// Test default
	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)

	assert.Equal(t, d1.Spec.Template.Spec.HostNetwork, false)
	assert.Equal(t, d1.Spec.Template.Spec.DNSPolicy, v1.DNSClusterFirst)

	// Test hostNetwork=true
	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-hostnetwork",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				HostNetwork: true,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.Equal(t, d2.Spec.Template.Spec.HostNetwork, true)
	assert.Equal(t, d2.Spec.Template.Spec.DNSPolicy, v1.DNSClusterFirstWithHostNet)
}

func TestDeploymentFilterLabels(t *testing.T) {
	excludedLabels := map[string]string{
		"foo":         "1",
		"app.foo.bar": "1",
	}

	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "my-instance",
			Labels: excludedLabels,
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{},
	}

	cfg := config.New(config.WithLabelFilters([]string{"foo*", "app.*.bar"}))

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	d, err := Deployment(params)
	require.NoError(t, err)

	assert.Len(t, d.ObjectMeta.Labels, 6)
	for k := range excludedLabels {
		assert.NotContains(t, d.ObjectMeta.Labels, k)
	}
}

func TestDeploymentFilterAnnotations(t *testing.T) {
	excludedAnnotations := map[string]string{
		"foo":         "1",
		"app.foo.bar": "1",
	}

	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-instance",
			Annotations: excludedAnnotations,
		},
		Spec: v1beta1.OpenTelemetryCollectorSpec{},
	}

	cfg := config.New(config.WithAnnotationFilters([]string{"foo*", "app.*.bar"}))

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	d, err := Deployment(params)
	require.NoError(t, err)

	assert.Len(t, d.ObjectMeta.Annotations, 4)
	for k := range excludedAnnotations {
		assert.NotContains(t, d.ObjectMeta.Annotations, k)
	}
}

func TestDeploymentNodeSelector(t *testing.T) {
	// Test default
	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)

	assert.Empty(t, d1.Spec.Template.Spec.NodeSelector)

	// Test nodeSelector
	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-nodeselector",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				HostNetwork: true,
				NodeSelector: map[string]string{
					"node-key": "node-value",
				},
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.Equal(t, d2.Spec.Template.Spec.NodeSelector, map[string]string{"node-key": "node-value"})
}

func TestDeploymentPriorityClassName(t *testing.T) {

	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)
	assert.Empty(t, d1.Spec.Template.Spec.PriorityClassName)

	priorityClassName := "test-class"

	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-priortyClassName",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				PriorityClassName: priorityClassName,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.Equal(t, priorityClassName, d2.Spec.Template.Spec.PriorityClassName)
}

func TestDeploymentAffinity(t *testing.T) {

	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)
	assert.Nil(t, d1.Spec.Template.Spec.Affinity)

	otelcol2 := v1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-priortyClassName",
		},
		Spec: v1beta1.AmazonCloudWatchAgent{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				Affinity: testAffinityValue,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.NotNil(t, d2.Spec.Template.Spec.Affinity)
	assert.Equal(t, *testAffinityValue, *d2.Spec.Template.Spec.Affinity)
}

func TestDeploymentTerminationGracePeriodSeconds(t *testing.T) {
	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)
	assert.Nil(t, d1.Spec.Template.Spec.TerminationGracePeriodSeconds)

	gracePeriodSec := int64(60)

	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-terminationGracePeriodSeconds",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				TerminationGracePeriodSeconds: &gracePeriodSec,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.NotNil(t, d2.Spec.Template.Spec.TerminationGracePeriodSeconds)
	assert.Equal(t, gracePeriodSec, *d2.Spec.Template.Spec.TerminationGracePeriodSeconds)
}

func TestDeploymentSetInitContainer(t *testing.T) {
	// prepare
	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-namespace",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				InitContainers: []v1.Container{
					{
						Name: "test",
					},
				},
			},
		},
	}
	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	// test

	d, err := Deployment(params)
	require.NoError(t, err)
	assert.Equal(t, "my-instance-collector", d.Name)
	assert.Equal(t, "my-instance-collector", d.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "true", d.Annotations["prometheus.io/scrape"])
	assert.Equal(t, "8888", d.Annotations["prometheus.io/port"])
	assert.Equal(t, "/metrics", d.Annotations["prometheus.io/path"])
	assert.Len(t, d.Spec.Template.Spec.InitContainers, 1)
}

func TestDeploymentTopologySpreadConstraints(t *testing.T) {
	// Test default

	otelcol1 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}
	d1 := Deployment(params1)
	assert.Equal(t, "my-instance", d1.Name)
	assert.Empty(t, d1.Spec.Template.Spec.TopologySpreadConstraints)

	// Test TopologySpreadConstraints
	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-topologyspreadconstraint",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				TopologySpreadConstraints: testTopologySpreadConstraintValue,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}
	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.Equal(t, "my-instance-topologyspreadconstraint-collector", d2.Name)
	assert.NotNil(t, d2.Spec.Template.Spec.TopologySpreadConstraints)
	assert.NotEmpty(t, d2.Spec.Template.Spec.TopologySpreadConstraints)
	assert.Equal(t, testTopologySpreadConstraintValue, d2.Spec.Template.Spec.TopologySpreadConstraints)
}

func TestDeploymentAdditionalContainers(t *testing.T) {
	// prepare
	otelcol := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-namespace",
		},

		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				AdditionalContainers: []v1.Container{
					{
						Name: "test",
					},
				},
			},
		},
	}
	cfg := config.New()

	params := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol,
		Log:     logger,
	}

	// test
	d, err := Deployment(params)
	require.NoError(t, err)
	assert.Equal(t, "my-instance-collector", d.Name)
	assert.Equal(t, "my-instance-collector", d.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "true", d.Annotations["prometheus.io/scrape"])
	assert.Equal(t, "8888", d.Annotations["prometheus.io/port"])
	assert.Equal(t, "/metrics", d.Annotations["prometheus.io/path"])
	assert.Len(t, d.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, v1.Container{Name: "test"}, d.Spec.Template.Spec.Containers[0])
}

func TestDeploymentShareProcessNamespace(t *testing.T) {
	// Test default
	otelcol1 := v1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	cfg := config.New()

	params1 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol1,
		Log:     logger,
	}

	d1, err := Deployment(params1)
	require.NoError(t, err)
	assert.False(t, *d1.Spec.Template.Spec.ShareProcessNamespace)

	// Test hostNetwork=true
	otelcol2 := v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance-with-shareprocessnamespace",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1beta1.AmazonCloudWatchAgentCommonFields{
				ShareProcessNamespace: true,
			},
		},
	}

	cfg = config.New()

	params2 := manifests.Params{
		Config:  cfg,
		OtelCol: otelcol2,
		Log:     logger,
	}

	d2, err := Deployment(params2)
	require.NoError(t, err)
	assert.True(t, *d2.Spec.Template.Spec.ShareProcessNamespace)
}
