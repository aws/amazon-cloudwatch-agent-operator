package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func main() {

	success := verifyInstrumentationEnvVariables("")
	if !success {
		fmt.Println("TestCase 2: Default Instrumentation Annotation Injection Test: FAIL")
		os.Exit(1)
	} else {
		fmt.Println("TestCase 2: Default Instrumentation Annotation Injection Test: PASS")
	}
	success = verifyInstrumentationEnvVariables("amazon-cloudwatch")
	if !success {
		fmt.Println("TestCase 3: Same Namespace Instrumentation Annotation Injection Test: FAIL")
		os.Exit(1)
	} else {
		fmt.Println("TestCase 3: Same Namespace Instrumentation Annotation Injection Test: PASS")
	}

}

func verifyInstrumentationEnvVariables(namespace string) bool {

	defaultJSONPath := "default_instrumentation_env_variables.json"
	namespacedJSONPath := "ns_stored_env_variables.json"

	jsonPath := defaultJSONPath

	var args []string
	if namespace != "" {
		jsonPath = namespacedJSONPath
		args = []string{"get", "pods", "-n", "nginx-namespace", "-l", "app=nginx", "-o=jsonpath='{.items[*].metadata.name}'"}
	} else {
		args = []string{"get", "pods", "-l", "app=nginx", "-o=jsonpath='{.items[*].metadata.name}'"}
	}

	//	// Define pod name and namespace
	cmd := "kubectl"

	// Execute kubectl command
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		fmt.Println("Error running kubectl command:", err)
		return false
	}

	// Process the output (remove quotes if present)
	podName := strings.ReplaceAll(string(out), "'", "")

	// Function to fetch environment variables from the pod
	envMap, err := getPodEnvVariables(podName, namespace)
	if err != nil {
		fmt.Println("Error fetching environment variables from the pod:", err)
		return false
	}
	fmt.Println("Pod environment variables:", envMap)

	// Read and parse JSON file containing key-value pairs
	fileData, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return false
	}

	var jsonData map[string]string
	if err := json.Unmarshal(fileData, &jsonData); err != nil {
		fmt.Println("Error parsing JSON file:", err)
		return false
	}
	fmt.Println("JSON data:", jsonData)
	// Compare environment variables with data from JSON file
	for key, value := range jsonData {
		if val, ok := envMap[key]; ok {
			if strings.ReplaceAll(val, " ", "") != strings.ReplaceAll(value, " ", "") {
				fmt.Printf("Mismatch: Key '%s' values do not match. Pod value: %s, JSON value: %s\n", key, val, value)
				return false
			} else {
				fmt.Printf("Match: Key '%s' values match. Pod value: %s, JSON value: %s\n", key, val, value)
			}
		} else {
			fmt.Printf("Key '%s' not found in pod environment variables\n", key)
			return false
		}
	}
	return true
}

// Function to fetch environment variables from a Kubernetes pod
func getPodEnvVariables(podName, namespace string) (map[string]string, error) {
	cmd := exec.Command("kubectl", "exec", "-it", podName, "-n", namespace, "--", "env")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	envVariables := strings.Split(string(output), "\n")
	envMap := make(map[string]string)

	// Parse environment variables into key-value pairs
	for _, envVar := range envVariables {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	return envMap, nil
}
