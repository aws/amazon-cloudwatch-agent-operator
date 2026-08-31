// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// languageDetector determines the application runtime language from container
// metadata available in the pod spec: image name, env vars, and command/args.
type languageDetector struct {
	logger logr.Logger
}

func newLanguageDetector(logger logr.Logger) *languageDetector {
	return &languageDetector{
		logger: logger,
	}
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

// detectContainer determines the language from pod-spec metadata only:
// image name patterns, env vars, and command/args.
func (d *languageDetector) detectContainer(container corev1.Container) instrumentation.Type {
	if lang := d.detectFromImageName(container.Image); lang != "" {
		return lang
	}
	if lang := d.detectFromEnvVars(container.Env); lang != "" {
		return lang
	}
	if lang := d.detectFromCommand(container.Command, container.Args); lang != "" {
		return lang
	}
	return ""
}

// detectFromImageName checks the container image reference string for language indicators.
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
