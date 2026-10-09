// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

func newTestDetector() *languageDetector {
	return &languageDetector{logger: logr.Discard()}
}

func TestClassifyEnv(t *testing.T) {
	d := newTestDetector()

	tests := []struct {
		name     string
		envName  string
		envValue string
		expected instrumentation.Type
	}{
		{"JAVA_HOME", "JAVA_HOME", "/usr/lib/jvm/java-17", instrumentation.TypeJava},
		{"JAVA_OPTS", "JAVA_OPTS", "-Xmx512m", instrumentation.TypeJava},
		{"CATALINA_HOME", "CATALINA_HOME", "/opt/tomcat", instrumentation.TypeJava},
		{"PYTHONPATH", "PYTHONPATH", "/app", instrumentation.TypePython},
		{"DJANGO_SETTINGS", "DJANGO_SETTINGS_MODULE", "myapp.settings", instrumentation.TypePython},
		{"FLASK_APP", "FLASK_APP", "app.py", instrumentation.TypePython},
		{"PYTHONUNBUFFERED", "PYTHONUNBUFFERED", "1", instrumentation.TypePython},
		{"NODE_ENV", "NODE_ENV", "production", instrumentation.TypeNodeJS},
		{"NODE_OPTIONS", "NODE_OPTIONS", "--max-old-space-size=4096", instrumentation.TypeNodeJS},
		{"NODE_VERSION", "NODE_VERSION", "20.11.0", instrumentation.TypeNodeJS},
		{"DOTNET_ROOT", "DOTNET_ROOT", "/usr/share/dotnet", instrumentation.TypeDotNet},
		{"ASPNETCORE_URLS", "ASPNETCORE_URLS", "http://+:8080", instrumentation.TypeDotNet},
		{"ASPNETCORE_ENVIRONMENT", "ASPNETCORE_ENVIRONMENT", "production", instrumentation.TypeDotNet},
		{"PATH with java", "PATH", "/usr/lib/jvm/bin:/usr/bin", instrumentation.TypeJava},
		{"PATH with python", "PATH", "/usr/local/bin/python:/usr/bin", instrumentation.TypePython},
		{"PATH with dotnet", "PATH", "/usr/share/dotnet:/usr/bin", instrumentation.TypeDotNet},
		{"PYTHON_VERSION", "PYTHON_VERSION", "3.11.15", instrumentation.TypePython},
		{"PYTHON_SHA256", "PYTHON_SHA256", "abc123", instrumentation.TypePython},
		{"PYTHON_PIP_VERSION", "PYTHON_PIP_VERSION", "23.0.1", instrumentation.TypePython},
		{"YARN_VERSION", "YARN_VERSION", "1.22.22", instrumentation.TypeNodeJS},
		{"generic env", "APP_PORT", "8080", ""},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.classifyEnv(tt.envName, tt.envValue)
			if result != tt.expected {
				t.Errorf("classifyEnv(%q, %q) = %q, want %q", tt.envName, tt.envValue, result, tt.expected)
			}
		})
	}
}

func TestDetectFromImageName(t *testing.T) {
	d := newTestDetector()

	tests := []struct {
		name     string
		image    string
		expected instrumentation.Type
	}{
		{"openjdk", "public.ecr.aws/docker/library/openjdk:17-slim", instrumentation.TypeJava},
		{"corretto", "amazoncorretto:17", instrumentation.TypeJava},
		{"tomcat", "tomcat:10-jdk17", instrumentation.TypeJava},
		{"java in ecr path", "978751493859.dkr.ecr.us-east-1.amazonaws.com/java-sample-app:latest", instrumentation.TypeJava},
		{"python", "public.ecr.aws/docker/library/python:3.11-slim", instrumentation.TypePython},
		{"django", "mycompany/django-app:latest", instrumentation.TypePython},
		{"node official", "node:20-alpine", instrumentation.TypeNodeJS},
		{"nodejs in name", "mycompany/nodejs-api:v2", instrumentation.TypeNodeJS},
		{"dotnet sdk", "mcr.microsoft.com/dotnet/sdk:8.0", instrumentation.TypeDotNet},
		{"aspnet", "mcr.microsoft.com/dotnet/aspnet:8.0", instrumentation.TypeDotNet},
		{"javascript not java", "mycompany/javascript-tools:latest", ""},
		{"ecr image with python in name", "978751493859.dkr.ecr.us-east-1.amazonaws.com/test-custom-python:latest", instrumentation.TypePython},
		{"truly opaque ecr image", "978751493859.dkr.ecr.us-east-1.amazonaws.com/service-abc:v2.3.1", ""},
		{"alpine", "alpine:3.19", ""},
		{"nginx", "nginx:1.25", ""},
		{"busybox", "busybox:latest", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.detectFromImageName(tt.image)
			if result != tt.expected {
				t.Errorf("detectFromImageName(%q) = %q, want %q", tt.image, result, tt.expected)
			}
		})
	}
}

func TestDetectFromCommand(t *testing.T) {
	d := newTestDetector()

	tests := []struct {
		name     string
		command  []string
		args     []string
		expected instrumentation.Type
	}{
		{"java command", []string{"java"}, []string{"-jar", "app.jar"}, instrumentation.TypeJava},
		{"java full path", []string{"/usr/bin/java"}, []string{"-jar", "app.jar"}, instrumentation.TypeJava},
		{"jar in args", []string{"sh", "-c"}, []string{"java -jar /app/service.jar"}, instrumentation.TypeJava},
		{"python command", []string{"python3"}, []string{"app.py"}, instrumentation.TypePython},
		{"python full path", []string{"/usr/local/bin/python"}, []string{"manage.py"}, instrumentation.TypePython},
		{"gunicorn", []string{"gunicorn"}, []string{"app:app"}, instrumentation.TypePython},
		{"uvicorn", []string{"uvicorn"}, []string{"main:app", "--host", "0.0.0.0"}, instrumentation.TypePython},
		{"flask", []string{"flask"}, []string{"run"}, instrumentation.TypePython},
		{".py file", []string{"python3"}, []string{"/app/main.py"}, instrumentation.TypePython},
		{"node command", []string{"node"}, []string{"server.js"}, instrumentation.TypeNodeJS},
		{"node full path", []string{"/usr/local/bin/node"}, []string{"index.js"}, instrumentation.TypeNodeJS},
		{"npm start", []string{"npm"}, []string{"start"}, instrumentation.TypeNodeJS},
		{"yarn", []string{"yarn"}, []string{"serve"}, instrumentation.TypeNodeJS},
		{".js file", []string{"node"}, []string{"/app/dist/main.js"}, instrumentation.TypeNodeJS},
		{".mjs file", []string{"node"}, []string{"app.mjs"}, instrumentation.TypeNodeJS},
		{"dotnet command", []string{"dotnet"}, []string{"MyApp.dll"}, instrumentation.TypeDotNet},
		{"dotnet full path", []string{"/usr/share/dotnet/dotnet"}, []string{"run"}, instrumentation.TypeDotNet},
		{".dll file", []string{"dotnet"}, []string{"/app/MyService.dll"}, instrumentation.TypeDotNet},
		{"sleep command", []string{"sleep"}, []string{"infinity"}, ""},
		{"shell command", []string{"sh", "-c"}, []string{"echo hello"}, ""},
		{"empty", []string{}, []string{}, ""},
		{"nginx", []string{"nginx"}, []string{"-g", "daemon off;"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.detectFromCommand(tt.command, tt.args)
			if result != tt.expected {
				t.Errorf("detectFromCommand(%v, %v) = %q, want %q", tt.command, tt.args, result, tt.expected)
			}
		})
	}
}

func TestDetectFromEnvVars_PodSpec(t *testing.T) {
	d := newTestDetector()

	tests := []struct {
		name     string
		env      []corev1.EnvVar
		expected instrumentation.Type
	}{
		{"JAVA_HOME", []corev1.EnvVar{{Name: "JAVA_HOME", Value: "/usr/lib/jvm/java-17"}}, instrumentation.TypeJava},
		{"PYTHONPATH", []corev1.EnvVar{{Name: "PYTHONPATH", Value: "/app"}}, instrumentation.TypePython},
		{"NODE_ENV", []corev1.EnvVar{{Name: "NODE_ENV", Value: "production"}}, instrumentation.TypeNodeJS},
		{"ASPNETCORE_URLS", []corev1.EnvVar{{Name: "ASPNETCORE_URLS", Value: "http://+:8080"}}, instrumentation.TypeDotNet},
		{"generic env", []corev1.EnvVar{{Name: "APP_PORT", Value: "8080"}}, ""},
		{"empty", []corev1.EnvVar{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.detectFromEnvVars(tt.env)
			if result != tt.expected {
				t.Errorf("detectFromEnvVars() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDetectContainer(t *testing.T) {
	d := newTestDetector()

	tests := []struct {
		name      string
		container corev1.Container
		expected  instrumentation.Type
	}{
		{
			name:      "detected from image name",
			container: corev1.Container{Image: "amazoncorretto:17"},
			expected:  instrumentation.TypeJava,
		},
		{
			name: "detected from env var",
			container: corev1.Container{
				Image: "123456789.dkr.ecr.us-east-1.amazonaws.com/my-app:latest",
				Env:   []corev1.EnvVar{{Name: "JAVA_HOME", Value: "/usr/lib/jvm/java-17"}},
			},
			expected: instrumentation.TypeJava,
		},
		{
			name: "detected from command",
			container: corev1.Container{
				Image:   "123456789.dkr.ecr.us-east-1.amazonaws.com/my-app:latest",
				Command: []string{"python3"},
				Args:    []string{"app.py"},
			},
			expected: instrumentation.TypePython,
		},
		{
			name:      "opaque image, no signals",
			container: corev1.Container{Image: "123456789.dkr.ecr.us-east-1.amazonaws.com/my-app:latest"},
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.detectContainer(tt.container)
			if result != tt.expected {
				t.Errorf("detectContainer() = %q, want %q", result, tt.expected)
			}
		})
	}
}
