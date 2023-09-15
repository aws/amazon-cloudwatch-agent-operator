#!/usr/bin/env bash

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

# Applying java-instrumentation SDK object
echo "Creating instrumentation object"
for i in {1..3}
do
    sdk_status=$(kubectl apply -f $INSTRUMENTATION_YAML 2>&1)
    if [[ "${sdk_status}" == *"Error "* ]];  then
        sleep 60
        continue
    fi
    break
done

sleep 60

echo "Install cloudwatch-agent as a daemon-set"
kubectl apply -f $AGENT_YAML

sleep 60

kubectl get pods -A