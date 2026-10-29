// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"os"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// defaultImageForType returns the operator's currently configured default auto-instrumentation
// image for a language. It reads the same environment variables getDefaultInstrumentation reads,
// which main.go sets from the --auto-instrumentation-<lang>-image flags.
//
// Its only purpose is to tell a superseded rollout's image from the current default:
// configuredImages feeds it to configuredImageMatch, which refuses to attribute a failure reported
// by a pod the operator has already moved past.
func defaultImageForType(instType instrumentation.Type) (string, bool) {
	var envVar string
	switch instType {
	case instrumentation.TypeJava:
		envVar = "AUTO_INSTRUMENTATION_JAVA"
	case instrumentation.TypePython:
		envVar = "AUTO_INSTRUMENTATION_PYTHON"
	case instrumentation.TypeDotNet:
		envVar = "AUTO_INSTRUMENTATION_DOTNET"
	case instrumentation.TypeNodeJS:
		envVar = "AUTO_INSTRUMENTATION_NODEJS"
	default:
		return "", false
	}
	image, ok := os.LookupEnv(envVar)
	if !ok || image == "" {
		return "", false
	}
	return image, true
}

// imageMatch is the outcome of comparing two image references. It is three-valued because the
// comparison genuinely has three answers, and an earlier boolean version had to fold the third
// into one of the other two: it reported "same" for a tag-only reference against a digest-only
// one, which share nothing but their repository, and "different" for an empty string, which states
// nothing at all. Both folds were wrong in the direction that loses a real failure.
//
// imageMatchInconclusive is the zero value, so a comparison that was never performed reads as
// "unknown" rather than as either verdict.
type imageMatch int

const (
	// imageMatchInconclusive means the two references have no field in common strong enough to
	// decide, or one of them is empty.
	imageMatchInconclusive imageMatch = iota
	// imageMatchSame means they refer to the same image, as far as compareImageRefs can tell.
	imageMatchSame
	// imageMatchDifferent means they refer to different images.
	imageMatchDifferent
)

// compareImageRefs compares an image string the KUBELET reported with an image string the operator
// was CONFIGURED with, and reports whether they are the same image, different images, or not
// comparable.
//
// The rules, in the order they are applied:
//
//   - either side empty -> INCONCLUSIVE. "The kubelet has not reported an image yet" is not
//     evidence of anything.
//   - different repository -> DIFFERENT.
//   - both sides carry a digest -> compare digests. A digest is the strongest identity either
//     side can state, so it decides even when the tags beside it differ.
//   - otherwise both sides carry a tag -> compare tags. A digest on ONE side does not conflict:
//     the kubelet routinely reports the configured tag resolved to a digest.
//   - neither side states a tag or a digest -> SAME. Both name the repository and nothing else.
//   - anything left -> INCONCLUSIVE. That is one side stating only a tag and the other only a
//     digest, or one side naming the bare repository while the other pins a tag or a digest.
//     "repo@sha256:old" against "repo:v2" is the case this exists for: the two have nothing in
//     common beyond the repository, and calling them the same image made an old pod look like it
//     was running the current default.
//
// What is compared is deliberately loose, because the two strings are produced by different
// writers and are routinely different spellings of one image:
//
//   - the kubelet may report a digest-resolved reference, "repo:tag@sha256:...", for a
//     configuration of plain "repo:tag";
//   - a container runtime expands a Docker Hub short name, so configured "python:3.11" comes back
//     as "docker.io/library/python:3.11";
//   - the same image may be pushed under several tags.
//
// Requiring the two strings to be equal would make configuredImageMatch call every pod superseded
// on any cluster whose runtime resolves digests or expands a short name, and attribution would then
// convict nothing at all. An earlier revision of the guard learned this the hard way: it recorded
// the kubelet's string at back-out time and later compared it against the configured string, which
// never matched, so the automatic retry it gated never fired. Nothing in this package may require
// that equality again.
//
// The registry host is dropped, so two different registries serving the same repository and tag
// compare SAME. That is a known and accepted heuristic: the precise alternative is resolving the
// applicable Instrumentation CR, which means re-running injection's whole selection logic inside
// the guard.
//
// Its one consumer is configuredImageMatch, and only a conclusive DIFFERENT makes it refuse to
// attribute a language's failure. INCONCLUSIVE proceeds with attribution and carries the ambiguity
// into the reason, so a pod whose image cannot be compared is still judged on its crash output
// rather than silently skipped; see configuredImageMatch and attribution.go's header.
func compareImageRefs(kubeletImage, configuredImage string) imageMatch {
	if kubeletImage == "" || configuredImage == "" {
		return imageMatchInconclusive
	}
	left, right := parseImageRef(kubeletImage), parseImageRef(configuredImage)
	if left.repository != right.repository {
		return imageMatchDifferent
	}
	switch {
	case left.digest != "" && right.digest != "":
		return sameOrDifferent(left.digest == right.digest)
	case left.tag != "" && right.tag != "":
		return sameOrDifferent(left.tag == right.tag)
	case left.tag == "" && right.tag == "" && left.digest == "" && right.digest == "":
		return imageMatchSame
	default:
		return imageMatchInconclusive
	}
}

// sameOrDifferent turns a comparison that WAS conclusive into its imageMatch.
func sameOrDifferent(equal bool) imageMatch {
	if equal {
		return imageMatchSame
	}
	return imageMatchDifferent
}

// imageRef is an image reference split into the parts compareImageRefs compares. The registry host
// is not kept: see compareImageRefs.
type imageRef struct {
	repository string
	tag        string
	digest     string
}

// parseImageRef splits an image reference into repository, tag and digest. It is not a validating
// parser; it only has to agree with itself on both sides of a comparison. An absent tag or digest
// is the empty string, which is what compareImageRefs reads as "this side does not state it".
func parseImageRef(ref string) imageRef {
	parsed := imageRef{}

	if at := strings.Index(ref, "@"); at >= 0 {
		parsed.digest = ref[at+1:]
		ref = ref[:at]
	}
	// A colon is only a tag separator when it comes after the last slash; before it, it is a
	// registry host's port.
	if colon := strings.LastIndex(ref, ":"); colon > strings.LastIndex(ref, "/") {
		parsed.tag = ref[colon+1:]
		ref = ref[:colon]
	}

	segments := strings.Split(ref, "/")
	if len(segments) > 1 && isRegistryHost(segments[0]) {
		segments = segments[1:]
	}
	// Docker Hub's implicit namespace: "python" and "library/python" are one repository.
	if len(segments) > 1 && segments[0] == "library" {
		segments = segments[1:]
	}
	parsed.repository = strings.Join(segments, "/")
	return parsed
}

// isRegistryHost reports whether an image reference's first path segment is a registry host rather
// than the first part of the repository name. A host carries a dot or a port, or is localhost.
func isRegistryHost(segment string) bool {
	return strings.Contains(segment, ".") || strings.Contains(segment, ":") || segment == "localhost"
}
