#!/bin/bash

kubectl apply -f ../sample-instrumentation.yaml
sleep 10
kubectl apply -f ../test-deployment.yaml
kubectl apply -f ../ns-annotation-test-deployment.yaml
sleep 60
kubectl get all -n amazon-cloudwatch
kubectl get pods -A
kubectl describe deployment amazon-cloudwatch-observability-controller-manager -n amazon-cloudwatch
pod_name="$(kubectl get pods -n amazon-cloudwatch -l app.kubernetes.io/component=amazon-cloudwatch-agent,app.kubernetes.io/instance=amazon-cloudwatch.cloudwatch-agent -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
status=$(kubectl get pod "$pod_name" -n amazon-cloudwatch -o=jsonpath='{.status.phase}')
if [ "$status" != "Running" ]; then
  echo "Pod $pod_name is not running. Exiting with ERROR."
  exit 1
else
  echo "Pod $pod_name is running. Continue with the workflow."
fi