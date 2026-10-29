// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

func TestDefaultImageForType(t *testing.T) {
	t.Setenv("AUTO_INSTRUMENTATION_PYTHON", testImagePython)
	t.Setenv("AUTO_INSTRUMENTATION_JAVA", "")

	image, ok := defaultImageForType(instrumentation.TypePython)
	assert.True(t, ok)
	assert.Equal(t, testImagePython, image)

	_, ok = defaultImageForType(instrumentation.TypeJava)
	assert.False(t, ok, "an empty environment variable is not a default image")

	_, ok = defaultImageForType(instrumentation.TypeGo)
	assert.False(t, ok, "go is not stamped and has no default image env var")
}

func TestCompareImageRefs(t *testing.T) {
	const bareRepository = "public.ecr.aws/aws-observability/adot-autoinstrumentation-python"

	for _, tt := range []struct {
		name       string
		kubelet    string
		configured string
		want       imageMatch
	}{
		{
			name:       "identical strings",
			kubelet:    testImagePython,
			configured: testImagePython,
			want:       imageMatchSame,
		},
		{
			name: "the kubelet resolved the tag to a digest",
			// The commonest real difference, and the one that made the retry path dead code. Only
			// one side carries a digest, so the tags both sides DO carry decide.
			kubelet:    testImagePython + "@sha256:f3b0c9a1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1",
			configured: testImagePython,
			want:       imageMatchSame,
		},
		{
			name: "the kubelet reported only a digest, with no tag on either side",
			// Nothing links a digest to a bare repository: the configured reference means
			// whatever :latest resolves to, and the digest may be any build of that repository.
			// This used to report "same", which let an old pod pass for one running the current
			// default.
			kubelet:    bareRepository + "@sha256:f3b0c9a1",
			configured: bareRepository,
			want:       imageMatchInconclusive,
		},
		{
			name: "a tag on one side and only a digest on the other",
			// The reported defect: "repo@sha256:old" compared equal to "repo:v2" on repository
			// alone, so a pod running a superseded image looked like one running the current
			// default - and in the other direction, an image the operator has moved past looked
			// like the one it configured.
			kubelet:    bareRepository + "@sha256:f3b0c9a1",
			configured: testImagePython,
			want:       imageMatchInconclusive,
		},
		{
			name:       "both sides name the repository alone",
			kubelet:    bareRepository,
			configured: bareRepository,
			want:       imageMatchSame,
		},
		{
			name:       "two different digests of the same tag",
			kubelet:    testImagePython + "@sha256:aaaa",
			configured: testImagePython + "@sha256:bbbb",
			want:       imageMatchDifferent,
		},
		{
			name: "the same digest under two different tags",
			// A digest is the strongest identity either side can state, so it decides even
			// against disagreeing tags: the same bytes were pushed twice.
			kubelet:    bareRepository + ":v0.0.1@sha256:aaaa",
			configured: bareRepository + ":v0.0.2@sha256:aaaa",
			want:       imageMatchSame,
		},
		{
			name:       "the runtime expanded a Docker Hub short name",
			kubelet:    "docker.io/library/python:3.11",
			configured: "python:3.11",
			want:       imageMatchSame,
		},
		{
			name:       "the runtime added only the registry host",
			kubelet:    "docker.io/myorg/python-instrumentation:v1",
			configured: "myorg/python-instrumentation:v1",
			want:       imageMatchSame,
		},
		{
			name:       "a registry with a port is a host, not a repository segment",
			kubelet:    "registry.example.com:5000/python-instrumentation:v1",
			configured: "python-instrumentation:v1",
			want:       imageMatchSame,
		},
		{
			name:       "same repository, different tag",
			kubelet:    bareRepository + ":v0.0.1",
			configured: bareRepository + ":v0.0.2",
			want:       imageMatchDifferent,
		},
		{
			name:       "different repository",
			kubelet:    testImagePython,
			configured: "my-registry/python-instrumentation:v0.0.1",
			want:       imageMatchDifferent,
		},
		{
			name: "the kubelet has reported no image yet",
			// An empty string states nothing, so it is not evidence that this pod is from a
			// superseded rollout. Reporting "different" here is what used to make such a pod skip
			// attribution altogether.
			kubelet:    "",
			configured: testImagePython,
			want:       imageMatchInconclusive,
		},
		{
			name:       "no configured default",
			kubelet:    testImagePython,
			configured: "",
			want:       imageMatchInconclusive,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, compareImageRefs(tt.kubelet, tt.configured))
			assert.Equal(t, tt.want, compareImageRefs(tt.configured, tt.kubelet),
				"the comparison must not depend on which side is which")
		})
	}
}

// TestCompareImageRefsNeverCallsATagTheSameAsADigest states the invariant on its own, because it
// is the defect this comparison was rewritten for: whatever else changes, a reference that pins
// only a tag and one that pins only a digest must never come back as the same image.
func TestCompareImageRefsNeverCallsATagTheSameAsADigest(t *testing.T) {
	for _, pair := range [][2]string{
		{"repo@sha256:old", "repo:v2"},
		{"repo:v2", "repo@sha256:old"},
		{"registry.example.com/repo@sha256:old", "repo:v2"},
	} {
		assert.NotEqual(t, imageMatchSame, compareImageRefs(pair[0], pair[1]), "%s vs %s", pair[0], pair[1])
	}
}
