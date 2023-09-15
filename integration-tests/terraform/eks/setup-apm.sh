#!/usr/bin/env bash

# Install Cert-Manager for EKS so cloudwatch-agent Operator can communicate with Cluster API Server via TLS
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.2/cert-manager.yaml

# Install cloudwatch-agent operator
echo "Creating cloudwatch-agent operator"
for i in {1..3}
do
    operator_status=$(kubectl apply -f $APM_YAML 2>&1)
    if [[ "${operator_status}" == *"Error "* ]];  then
        sleep 60
        continue
    fi
    break
done
sleep 60

echo "Install cloudwatch-agent as a daemon-set"
kubectl apply -f $AGENT_YAML

echo "Waiting for 1 minute for the pods to start-up"
sleep 60