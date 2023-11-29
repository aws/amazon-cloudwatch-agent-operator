#!/bin/bash


# Variables
POD_NAME="$(kubectl get pods -n amazon-cloudwatch -l app=nginx -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
NAMESPACE="amazon-cloudwatch"

# Function to fetch and store environment variables from the pod
fetch_env_variables() {
    # Fetch environment variables from the pod
    ENV_VARIABLES=$(kubectl exec -it "$POD_NAME" -n "$NAMESPACE" -- printenv)

    # Store environment variables in key-value pair
    while IFS= read -r line; do
        echo "$line" | awk -F= '{print $1 "=" $2}' >> pod_env_variables.txt
    done <<< "$ENV_VARIABLES"
}

# Function to compare stored environment variables
compare_env_variables() {
    # Fetch environment variables again
    fetch_env_variables

    # Compare stored variables with current variables
    if cmp -s stored_env_variables.txt pod_env_variables.txt; then
        echo "Environment variables are unchanged."
    else
        echo "Changes detected in environment variables."
    fi
}

# Check if stored_env_variables.json exists
if [ -f stored_env_variables.txt ]; then
    compare_env_variables
else
    echo "No stored environment variables found. Fetching and storing..."
    fetch_env_variables
    mv pod_env_variables.txt stored_env_variables.txt
fi