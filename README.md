# Amazon CloudWatch Agent Operator
The Amazon CloudWatch Agent Operator is software developed to manage the [CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html) on kubernetes.

Supported Languages:
- Java
- Python
- .NET
- NodeJS

This repo is based off of the [OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator)

## Build and Deployment
- Image can be built using `make container`
- Deploy kubernetes objects to your cluster `make deploy`

## Pre requisites
1. Have an existing kubernetes cluster, such as [minikube](https://minikube.sigs.k8s.io/docs/start/)

2. Install cert-manager on your cluster
```
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
```

## Getting started
1. Set a shortcut for kubectl for the operator namespace

```
kubectl config set-context --current --namespace=amazon-cloudwatch
```

2. Look at all resources created

```
kubectl get all
```

3. Look at the manager pod logs to ensure the manager is functioning and waiting for workers

```
kubectl logs amazon-cloudwatch-agent-operator-controller-manager-66f67f47f78
```

You should see logs that look similar to below

```
{"level":"info","ts":"2023-06-29T01:37:36Z","msg":"Starting workers","controller":"amazoncloudwatchagent","controllerGroup":"cloudwatch.aws.amazon.com","controllerKind":"AmazonCloudWatchAgent","worker count":1}
```

4. Create an AmazonCloudWatchAgent resource

```
kubectl apply -f - <<EOF
apiVersion: cloudwatch.aws.amazon.com/v1alpha1
kind: AmazonCloudWatchAgent
metadata:
  name: cloudwatch-agent
  namespace: amazon-cloudwatch
spec:
  mode: daemonset
  serviceAccount: cloudwatch-agent
  config: |
    {
        // insert cloudwatch agent config here
    }
  volumeMounts:
  - mountPath: /rootfs
    name: rootfs
    readOnly: true
  - mountPath: /var/run/docker.sock
    name: dockersock
    readOnly: true
  - mountPath: /run/containerd/containerd.sock
    name: containerdsock
  - mountPath: /var/lib/docker
    name: varlibdocker
    readOnly: true
  - mountPath: /sys
    name: sys
    readOnly: true
  - mountPath: /dev/disk
    name: devdisk
    readOnly: true
  volumes:
  - name: rootfs
    hostPath:
      path: /
  - hostPath:
      path: /var/run/docker.sock
    name: dockersock
  - hostPath:
      path: /var/lib/docker
    name: varlibdocker
  - hostPath:
      path: /run/containerd/containerd.sock
    name: containerdsock
  - hostPath:
      path: /sys
    name: sys
  - hostPath:
      path: /dev/disk/
    name: devdisk
  env:
    - name: K8S_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: HOST_IP
      valueFrom:
        fieldRef:
          fieldPath: status.hostIP
    - name: HOST_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: K8S_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
EOF
```

5. Create Instrumentation resource

```
kubectl apply -f - <<EOF
apiVersion: cloudwatch.aws.amazon.com/v1alpha1
kind: Instrumentation
metadata:
  name: java-instrumentation
  namespace: default # use a namespace with pods you'd like to inject
spec:
  exporter:
    endpoint: http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics
  propagators:
    - tracecontext
    - baggage
    - b3
    - xray
  java:
    env:
      - name: OTEL_METRICS_EXPORTER
        value: "none"
      - name: OTEL_LOGS_EXPORTER
        value: "none"
      - name: OTEL_AWS_APPLICATION_SIGNALS_ENABLED
        value: "true"
      - name: OTEL_EXPORTER_OTLP_PROTOCOL
        value: "http/protobuf"
      - name: OTEL_AWS_APPLICATION_SIGNALS_EXPORTER_ENDPOINT
        value: "http://cloudwatch-agent.amazon-cloudwatch:4316/v1/metrics"
EOF
```

## Helpful tools
1. This package uses [kubebuilder markers](https://book.kubebuilder.io/reference/markers.html) to generate kubernetes configs. Run `make manifests` to create crds and roles in `config/crd` and `config/rbac`
2. Generate deepcopy.go by running `make generate`


## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
