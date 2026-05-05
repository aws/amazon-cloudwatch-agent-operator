// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// languageDetector inspects container image configs from the registry to determine
// the application runtime language. It reads ENV, CMD, and ENTRYPOINT from the
// image manifest without pulling layers.
type languageDetector struct {
	logger   logr.Logger
	keychain authn.Keychain
	timeout  time.Duration
}

func newLanguageDetector(logger logr.Logger) *languageDetector {
	return &languageDetector{
		logger:   logger,
		keychain: authn.NewMultiKeychain(newECRKeychain(logger), authn.DefaultKeychain),
		timeout:  5 * time.Second,
	}
}

// ecrKeychain uses the AWS SDK default credential chain (instance profile, IRSA, env vars)
// to authenticate with Amazon ECR. This is the same credential chain customers configure
// via aws configure / IAM roles when setting up EKS.
type ecrKeychain struct {
	logger logr.Logger
}

func newECRKeychain(logger logr.Logger) *ecrKeychain {
	return &ecrKeychain{logger: logger}
}

func (k *ecrKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	registry := resource.RegistryStr()
	if !strings.Contains(registry, ".dkr.ecr.") || !strings.Contains(registry, ".amazonaws.com") {
		return authn.Anonymous, nil
	}

	sess, err := session.NewSession()
	if err != nil {
		k.logger.V(2).Info("could not create AWS session for ECR auth", "error", err)
		return authn.Anonymous, nil
	}

	svc := ecr.New(sess)
	result, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		k.logger.V(2).Info("could not get ECR authorization token", "error", err)
		return authn.Anonymous, nil
	}

	if len(result.AuthorizationData) == 0 {
		return authn.Anonymous, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(*result.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return authn.Anonymous, nil
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return authn.Anonymous, nil
	}

	return authn.FromConfig(authn.AuthConfig{
		Username: parts[0],
		Password: parts[1],
	}), nil
}

// detectLanguages inspects all containers in a pod template and returns the set of
// detected languages. Returns an empty set if no language can be confidently determined.
func (d *languageDetector) detectLanguages(podSpec *corev1.PodTemplateSpec) instrumentation.TypeSet {
	detected := make(instrumentation.TypeSet)

	for _, container := range podSpec.Spec.Containers {
		if lang := d.detectContainer(container); lang != "" {
			d.logger.V(2).Info("detected language from container",
				"container", container.Name, "image", container.Image, "language", lang)
			detected[lang] = nil
		}
	}

	return detected
}

// detectContainer fetches the image config from the registry and inspects it.
// Falls back to pod-spec-only detection if the registry fetch fails.
func (d *languageDetector) detectContainer(container corev1.Container) instrumentation.Type {
	// Try fetching real image config from registry
	if cfg := d.fetchImageConfig(container.Image); cfg != nil {
		if lang := d.detectFromConfig(cfg); lang != "" {
			return lang
		}
	}

	// Fallback: check image name for language patterns (handles private registries where config fetch fails)
	if lang := d.detectFromImageName(container.Image); lang != "" {
		return lang
	}

	// Fallback: check pod-spec-level env vars and commands
	if lang := d.detectFromEnvVars(container.Env); lang != "" {
		return lang
	}
	if lang := d.detectFromCommand(container.Command, container.Args); lang != "" {
		return lang
	}
	return ""
}

// detectFromImageName checks the container image reference string for language indicators.
// This is a heuristic fallback for when registry config fetch is not available.
func (d *languageDetector) detectFromImageName(image string) instrumentation.Type {
	lower := strings.ToLower(image)

	javaPatterns := []string{
		"openjdk", "jdk", "jre", "eclipse-temurin", "amazoncorretto",
		"corretto", "adoptopenjdk", "ibm-semeru", "graalvm",
		"tomcat", "jetty", "wildfly", "quarkus", "springboot",
		"spring-boot", "maven", "gradle", "libertycore", "payara",
	}
	for _, p := range javaPatterns {
		if strings.Contains(lower, p) {
			return instrumentation.TypeJava
		}
	}
	if strings.Contains(lower, "java") && !strings.Contains(lower, "javascript") {
		return instrumentation.TypeJava
	}

	pythonPatterns := []string{
		"python", "django", "flask", "fastapi", "uvicorn",
		"gunicorn", "celery", "conda", "miniconda", "anaconda",
	}
	for _, p := range pythonPatterns {
		if strings.Contains(lower, p) {
			return instrumentation.TypePython
		}
	}

	nodePatterns := []string{
		"node:", "/node:", "nodejs", "node-", "-node",
		"express", "nextjs", "next.js", "nestjs",
	}
	for _, p := range nodePatterns {
		if strings.Contains(lower, p) {
			return instrumentation.TypeNodeJS
		}
	}

	dotnetPatterns := []string{
		"dotnet", "aspnet", "asp.net", "mcr.microsoft.com/dotnet",
	}
	for _, p := range dotnetPatterns {
		if strings.Contains(lower, p) {
			return instrumentation.TypeDotNet
		}
	}

	return ""
}

// fetchImageConfig retrieves the image config (ENV, CMD, ENTRYPOINT, Labels) from the
// registry. Only fetches the manifest and config blob — no layer data is downloaded.
func (d *languageDetector) fetchImageConfig(imageRef string) *v1.Config {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		d.logger.V(2).Info("could not parse image reference", "image", imageRef, "error", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	desc, err := remote.Get(ref,
		remote.WithAuthFromKeychain(d.keychain),
		remote.WithContext(ctx),
	)
	if err != nil {
		d.logger.V(2).Info("could not fetch image descriptor", "image", imageRef, "error", err)
		return nil
	}

	img, err := desc.Image()
	if err != nil {
		d.logger.V(2).Info("could not get image from descriptor", "image", imageRef, "error", err)
		return nil
	}

	cfgFile, err := img.ConfigFile()
	if err != nil {
		d.logger.V(2).Info("could not read image config", "image", imageRef, "error", err)
		return nil
	}

	return &cfgFile.Config
}

// detectFromConfig inspects the image config's ENV, ENTRYPOINT, CMD, and Labels.
func (d *languageDetector) detectFromConfig(cfg *v1.Config) instrumentation.Type {
	// Check image-level environment variables
	if lang := d.detectFromImageEnv(cfg.Env); lang != "" {
		return lang
	}

	// Check ENTRYPOINT and CMD
	if lang := d.detectFromCommand(cfg.Entrypoint, cfg.Cmd); lang != "" {
		return lang
	}

	return ""
}

// detectFromImageEnv checks environment variables from the image config (string slice format: "KEY=VALUE").
func (d *languageDetector) detectFromImageEnv(envVars []string) instrumentation.Type {
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) < 2 {
			continue
		}
		envName := strings.ToUpper(parts[0])
		envValue := strings.ToLower(parts[1])

		if lang := d.classifyEnv(envName, envValue); lang != "" {
			return lang
		}
	}
	return ""
}

// detectFromEnvVars checks environment variables from the pod spec (corev1.EnvVar format).
func (d *languageDetector) detectFromEnvVars(envVars []corev1.EnvVar) instrumentation.Type {
	for _, env := range envVars {
		envName := strings.ToUpper(env.Name)
		envValue := strings.ToLower(env.Value)

		if lang := d.classifyEnv(envName, envValue); lang != "" {
			return lang
		}
	}
	return ""
}

// classifyEnv determines the language from an env var name and value.
func (d *languageDetector) classifyEnv(name, value string) instrumentation.Type {
	switch name {
	case "JAVA_HOME", "JAVA_TOOL_OPTIONS", "JAVA_OPTS",
		"JVM_OPTS", "CATALINA_HOME", "CATALINA_OPTS",
		"MAVEN_HOME", "GRADLE_HOME":
		return instrumentation.TypeJava
	}

	switch name {
	case "PYTHONPATH", "PYTHONHOME", "PYTHONDONTWRITEBYTECODE",
		"PYTHONUNBUFFERED", "PIP_NO_CACHE_DIR",
		"PYTHON_VERSION", "PYTHON_SHA256", "PYTHON_PIP_VERSION",
		"DJANGO_SETTINGS_MODULE", "FLASK_APP":
		return instrumentation.TypePython
	}

	switch name {
	case "NODE_PATH", "NODE_ENV", "NODE_OPTIONS",
		"NPM_CONFIG_PREFIX", "YARN_CACHE_FOLDER",
		"NODE_VERSION", "YARN_VERSION":
		return instrumentation.TypeNodeJS
	}

	switch name {
	case "DOTNET_ROOT", "ASPNETCORE_URLS", "ASPNETCORE_ENVIRONMENT",
		"DOTNET_RUNNING_IN_CONTAINER", "DOTNET_SYSTEM_GLOBALIZATION_INVARIANT",
		"NUGET_PACKAGES", "CORECLR_ENABLE_PROFILING":
		return instrumentation.TypeDotNet
	}

	if name == "PATH" {
		if strings.Contains(value, "/usr/lib/jvm") || strings.Contains(value, "java") {
			return instrumentation.TypeJava
		}
		if strings.Contains(value, "python") {
			return instrumentation.TypePython
		}
		if strings.Contains(value, "/usr/local/lib/node") || strings.Contains(value, "nodejs") {
			return instrumentation.TypeNodeJS
		}
		if strings.Contains(value, "dotnet") {
			return instrumentation.TypeDotNet
		}
	}
	return ""
}

// detectFromCommand checks entrypoint and command args for language indicators.
func (d *languageDetector) detectFromCommand(command []string, args []string) instrumentation.Type {
	allParts := append(command, args...)
	if len(allParts) == 0 {
		return ""
	}

	for _, part := range allParts {
		lower := strings.ToLower(part)

		if lower == "java" || strings.HasSuffix(lower, "/java") ||
			strings.HasSuffix(lower, ".jar") ||
			strings.Contains(lower, "-javaagent:") ||
			strings.Contains(lower, "org.apache.catalina") ||
			strings.Contains(lower, "org.springframework") {
			return instrumentation.TypeJava
		}

		if lower == "python" || lower == "python3" || lower == "python2" ||
			strings.HasSuffix(lower, "/python") || strings.HasSuffix(lower, "/python3") ||
			strings.HasSuffix(lower, ".py") ||
			lower == "gunicorn" || lower == "uvicorn" || lower == "celery" ||
			lower == "django-admin" || lower == "flask" {
			return instrumentation.TypePython
		}

		if lower == "node" || lower == "nodejs" ||
			strings.HasSuffix(lower, "/node") ||
			strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".mjs") ||
			lower == "npm" || lower == "yarn" || lower == "npx" || lower == "pnpm" {
			return instrumentation.TypeNodeJS
		}

		if lower == "dotnet" || strings.HasSuffix(lower, "/dotnet") ||
			strings.HasSuffix(lower, ".dll") {
			return instrumentation.TypeDotNet
		}
	}

	return ""
}
