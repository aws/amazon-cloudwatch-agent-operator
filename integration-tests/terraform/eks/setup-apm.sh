#!/usr/bin/env bash

# Install Cert-Manager for EKS so cloudwatch-agent Operator can communicate with Cluster API Server via TLS
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.2/cert-manager.yaml

echo "Install cloudwatch-agent operator"
for i in {1..3}
do
    operator_status=$(kubectl apply -f $APM_YAML 2>&1)
    if [[ "${operator_status}" == *"Error "* ]];  then
        sleep 60
        continue
    fi
    break
done

echo "Waiting for 1 minute for the resources to start-up"
sleep 60

echo "Install instrumentation object"
for i in {1..3}
do
    sdk_status=$(kubectl apply -f $INSTRUMENTATION_YAML 2>&1)
    if [[ "${sdk_status}" == *"Error "* ]];  then
        sleep 60
        continue
    fi
    break
done

echo "Install cloudwatch-agent as a daemon-set"
kubectl apply -f $AGENT_YAML

echo "Waiting for 1 minute for the resources to start-up"
sleep 60
kubectl get pods -A
