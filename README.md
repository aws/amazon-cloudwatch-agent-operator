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

## Instrumentation guard

Auto-instrumentation adds an init container and a set of environment variables to a pod. If that
breaks the application - a crash loop, a container that never starts - the workload is down for a
reason the customer did not ask for, and nothing in the operator notices. The instrumentation guard
watches the pods the operator injected and, when one of them breaks shortly after starting **and
the crash output holds auto-instrumentation responsible**, says so in a Kubernetes `Warning` Event
on the workload that owns it.

**The guard is detect-only.** Precisely:

- It **does** watch auto-instrumented pods, read the crash output the kubelet copied into pod
  status, decide whether the pod is broken, decide whether auto-instrumentation is to blame, and
  emit an Event on the owning workload naming the pod, the languages and images involved, why it
  thinks the pod is broken, why it blames injection, and which pod template annotations the
  customer can set to `"false"` to stop the injection.
- It **does not** modify any customer object. No pod template patch, no annotation written, no
  injection disabled, no pod deleted, no pod evicted, and no restart. There is no mode that does
  any of those; see [Flags](#flags).

It stops there deliberately. Attribution is pattern matching on crash text, and innocent crash
output can be frame-for-frame as incriminating as guilty crash output - see
[Known limits](#known-limits) for the cases that are known to fool it. Disabling instrumentation on
a false positive would cost a customer their telemetry silently and for good, so the decision stays
with the person who can read the application; the operator only brings the failure to their
attention.

It is two parts.

**1. The stamp, in the pod mutating webhook.** After injection, the operator labels the pod
`cloudwatch.aws.amazon.com/auto-instrumented: "true"` and sets
`terminationMessagePolicy: FallbackToLogsOnError` on each container it mounted the
auto-instrumentation payload into. The label is a server-side watch filter, so the operator caches
and reconciles only instrumented pods rather than every pod in the cluster. The policy is what makes
the crash output readable: with it, the kubelet copies the tail of the container's log into
`status.containerStatuses[].lastState.terminated.message` when the container exits with an error.

**2. Detect, attribute, report, in the controller.** For a labelled pod, the guard first asks
whether it is broken - an application container past the restart threshold or OOMKilled, or an
injected init container that failed, restarted too often or cannot pull its image, all inside the
detection window. Then it asks whether that is the operator's doing. Both must say yes before
anything is emitted, and an Event is all that is ever emitted: one `Warning` with reason
`InstrumentationSuspected` on the owning Deployment, StatefulSet or DaemonSet, carrying the pod, the
injected languages and images, both halves of the verdict, the inject annotations to set to
`"false"`, and the fact that the finding is pattern matching and can be wrong. On a workload that
does not replace its own pods when its template changes, a second `Warning`,
`ManualPodDeletionRequired`, lists the pods that would have to be deleted by hand for such a change
to take effect - see [Known limits](#known-limits).

### Attributing a failure

A failure is the operator's when either of two things is true. Structurally, when one of the init
containers the operator injected failed or cannot pull its image: that container's name, image,
command and volume are all operator-set, so without injection there would be no such container to
fail, and no text is parsed. That argument covers a failure the container caused, so it is not
applied to one inflicted on it from outside: a pod that is terminating, or an injected init
container killed by a signal (exit `128 + signal`, so 137 and 143), is never attributed to the
operator, since a drain, an eviction, a preemption or a rolling update kills containers for reasons
of its own. An OOM kill is the exception and still counts, because the memory limit it exceeded is
one the operator sets. Evidentially, when a traceback frame in the crash output names a file
inside the injected mount path **and** its function is exactly `<module>` - the payload died while
it was being imported, so nothing of the application's had to call it.

A traceback that runs through ADOT's own loader is never the operator's, even when it ends on a
`<module>` frame in the injected path. The loader catches an instrumentor that fails to import,
logs the whole traceback and carries on with the next one, and `site.execsitecustomize()` swallows
anything that escapes `sitecustomize` outright - so the application kept running, and the traceback
is in the output only because the loader printed it.

That exemption applies to **one traceback**, not to the whole message, because one message
routinely holds several. The failure this guard was built for prints both at once: the version
mismatch breaks an ADOT instrumentor's import, which the loader absorbs and logs, and then breaks
the application's own import of the library the payload shadows, which nothing absorbs. The guard
therefore splits each message on `Traceback (most recent call last):` and judges every traceback
on its own, so an absorbed failure cannot excuse a fatal one printed beside it. Text before the
first such header is the remains of a traceback whose head the kubelet's truncation cut off, and it
never convicts on its own: the lines the cut removed may be exactly the loader frames that excused
it.

A language is not attributed when its injected image is demonstrably not the operator's configured
default for it. A pod left over from an earlier rollout therefore cannot disable injection the
operator has already moved past. The comparison is on the repository and tag or digest, not on the
exact string, because the image the kubelet reports is routinely a different spelling of the one the
operator was configured with. It has three outcomes rather than two: when the two references share
nothing that can be compared - one pins a digest and the other a tag, or the kubelet has not
reported an image yet - the guard attributes the failure anyway and says in the Event that the image
could not be compared. Staying silent there would leave a crash-looping application with no signal
at all, while reporting it costs one Event on a workload that is already broken.

The function name is what carries the decision, not the path. The injected path appears in the
output of perfectly healthy pods, because the ADOT exporter prints full tracebacks whenever it
cannot reach a collector; those are pass-through call frames of already-imported code, and the guard
deliberately ignores them. A real capture from a healthy run that exited 0 contained 21 frames
naming the injected path.

`terminationMessagePolicy: FallbackToLogsOnError` is the only change the operator makes to the
customer's own container beyond the injection itself, and it is benign: it changes nothing about how
the container runs, only what the kubelet reports after it exits with an error, and an application
that writes to `/dev/termination-log` still gets its own message, because that file takes precedence
over the fallback.

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `--instrumentation-guard-mode` | `dry-run` | Whether the guard runs. `off` is a complete kill switch: no controller is registered, and the pod mutating webhook does not stamp injected pods either, so they get neither the `cloudwatch.aws.amazon.com/auto-instrumented` label nor the `terminationMessagePolicy` change. `dry-run` watches, attributes and emits Events, and writes nothing. These are the only two values; there is no mode in which the guard modifies a workload, and an operator started with the removed `on` value fails to start rather than running with a behaviour it was not asked for. |
| `--instrumentation-guard-restart-threshold` | `3` | Container restart count at which an auto-instrumented pod counts as broken. |
| `--instrumentation-guard-window` | `10m` | How long after a pod starts a failure is attributed to auto-instrumentation. |
| `--instrumentation-guard-image-pull-patience` | `5m` | How long an injected init container may sit unable to pull its image before the pod counts as broken. A pull failure gets more patience than any other trigger, because it is often a transient registry problem rather than a broken image. |

`dry-run` is both the default and the furthest the guard goes, so the only thing an upgrade adds to
a cluster is Events. `off` is there for a cluster that does not want even those: it registers no
controller, so nothing is evaluated and no Event is emitted.

### Label and annotations

Set by the pod mutating webhook on every pod the operator injects, unless the guard is `off` - the
label and the `terminationMessagePolicy` change exist only for the guard, so with the guard off
neither is applied:

- `cloudwatch.aws.amazon.com/auto-instrumented: "true"` (label) - this pod got
  auto-instrumentation. It is a label rather than an annotation so that it can be a server-side
  watch selector: it is what lets the operator watch and cache only instrumented pods.

The webhook writes no annotations. The injected image per language is read back from
`status.initContainerStatuses`, where the kubelet already reports it, so the earlier
`cloudwatch.aws.amazon.com/injected-<lang>-image` annotations are gone.

**The guard writes no annotations at all** - not on the pod, not on the pod template, not on the
workload. It reads one:

- `cloudwatch.aws.amazon.com/instrumentation-guard` on a **workload's own metadata** silences the
  guard for that workload. The operator never writes it; a customer who has read the finding and
  decided to keep auto-instrumentation anyway adds it, and the guard then emits nothing further
  about that workload. Any value will do, and deleting it puts the workload back in scope. It
  silences the guard only **while injection is still turned off** on the pod template for the
  languages in the pod: if `instrumentation.opentelemetry.io/inject-<lang>` comes back to `"true"`
  - which `kubectl apply`, Argo CD and Flux all do, none of them removing the record - the guard
  reports again, because the workload is being injected again. The documented JSON shape, for a
  value worth reading later, is `{"backedOutAt": <RFC3339 time>, "reason": <string>, "failedImages": {<lang>: <image>},
  "previous": {<inject annotation key>: <value or null>}}`.

### What to do when the guard reports a workload

The guard changes nothing, so every step here is the customer's, and the
`InstrumentationSuspected` Event names the keys involved:

1. Read the crash output the Event is quoting, in
   `kubectl get pod <pod> -o jsonpath='{.status.containerStatuses[*].lastState.terminated.message}'`.
   The guard's verdict is pattern matching and the known false positives below are real, so this is
   the step that decides whether it is right.
2. If auto-instrumentation is the cause, set `instrumentation.opentelemetry.io/inject-<lang>:
   "false"` on the workload's **pod template**, for each language named in the Event. The
   application then rolls out without instrumentation, with no telemetry, instead of crash-looping.
   If the workload is one that does not replace its own pods, the `ManualPodDeletionRequired` Event
   lists the pods to delete for that change to take effect. On a **paused** Deployment that Event
   asks for `kubectl rollout resume` instead: while it is paused the template change does not roll
   out, and deleting its pods only has the existing ReplicaSet recreate them from the template that
   still injects.
   Having disabled injection, add the `cloudwatch.aws.amazon.com/instrumentation-guard` annotation
   to the workload if you want no further Events about it while it stays disabled.
3. If it is not the cause, please report the false positive. The guard keeps reporting a workload
   that is still being injected - the record annotation does not silence a workload whose
   `inject-<lang>` annotations are back to `"true"`, because the operator cannot tell that state
   apart from a workload nobody has looked at yet.

Nothing about this is permanent from the operator's side: it never disabled anything, so there is
nothing for it to re-enable, and a workload whose inject annotations are restored is injected again
on its next rollout.

### Known limits

- **Attribution can be wrong, and the guard does not act on its own because of it.** The rule is
  text matching on a crash message, and these are the cases that are known to fool it into blaming
  injection for a failure that is not its fault. Each of them costs a wrong Event, which is why
  that is all a finding is:
  - **Exception chaining.** A chained Python exception (`raise ... from ...`, or an exception
    raised while another was being handled) prints as several traceback blocks. The guard judges
    each block separately, so a payload frame can end up in a different block from the ADOT loader
    frame that would have excused it, and the block it lands in reads as fatal.
  - **A failure the application caught.** An application whose own `try`/`except` absorbs the
    import error can still print the traceback itself - with `traceback.print_exc()`, or through a
    logger - and then carry on or die later of something unrelated. The text looks the same as a
    fatal one; whether the interpreter died of it is not in the message.
  - **A failure on a worker thread.** An exception on a thread prints a full traceback and kills
    only that thread. If the process later crashes for its own reasons, the kept tail of the log
    can hold the thread's traceback and nothing about the real cause.
  - **A loader frame the parser cannot read.** The exemption for ADOT's own loader depends on
    recognising its frames, and the frame regex needs the `, in <func>` suffix that CPython
    normally prints. A frame line without it is not parsed, so a traceback that genuinely ran
    through the loader can lose the evidence that would have cleared it.
- **Some workloads do not replace their own pods, and the guard never deletes one.** Disabling
  injection means changing the pod template; whether that reaches the running pods is the workload
  controller's decision. Measured on Kubernetes v1.34.8: a Deployment recovers on either strategy,
  and so does a DaemonSet on the default `RollingUpdate`, but a DaemonSet with
  `updateStrategy: OnDelete` does **not**, and neither does a **StatefulSet on any strategy,
  including the default**. A Deployment with `spec.paused: true` does not either: it creates no new
  ReplicaSet for the change, and deleting its pods does not help because the ReplicaSet it already
  has recreates them from the old template, so its `ManualPodDeletionRequired` Event asks for a
  `kubectl rollout resume` rather than a deletion. A StatefulSet's default pod management policy is `OrderedReady`, so its
  controller waits for the pod it just replaced to become Ready before touching the next one - and
  the broken pod never becomes Ready. A partitioned `RollingUpdate` has the same shape for any pod
  below the partition. In all of those cases the guard emits a second `Warning` Event,
  `ManualPodDeletionRequired`, with the `kubectl delete pod` command to run after disabling
  injection. It names every auto-instrumented pod of the workload, not just the one that broke,
  because on an `OnDelete` workload nothing is replaced until each of them is deleted; the pod the
  guard found comes first, and beyond five the Event says how many more there are, so the message
  stays within the Event size the API accepts. The application stays down until someone runs the
  command.
- **An image pinned in an `Instrumentation` CR is not reported on.** Attribution does not apply to
  a language whose injected image is conclusively not the operator's configured default, so a
  CR-pinned image that can be told apart from the default never produces a finding. That is the
  same rule that stops a pod from a superseded rollout producing one about an image the operator
  has already moved past. Resolving the applicable `Instrumentation` CR inside the guard would mean
  re-running injection's whole selection logic, which is why the guard compares against the
  configured default instead. A CR-pinned image the comparison cannot tell apart from the default -
  it pins a digest where the default pins a tag, say - is attributed, with the ambiguity named in
  the Event.
- **Message attribution is Python-only.** Only Python prepends the payload root to the
  application's own import path, so only Python can shadow the application's libraries. Java,
  Node.js and .NET rely on the structural signal alone: an injected init container that failed.
- **Truncation can hide a real failure.** The kubelet caps the message at 2048 bytes and keeps the
  **tail**. More than about 2KB of output printed after a genuine fatal traceback pushes the
  `<module>` frames out of the message, and the failure is missed. That is a false negative: the
  guard does nothing and the application stays broken. A chatty application is the risk. The same
  cut can also leave a guilty traceback's `<module>` frames in place while removing the
  `Traceback (most recent call last):` header above them, and the guard abstains there too, for
  the same reason: without the header it cannot tell whether the lines the cut took were the loader
  frames that would have excused those frames.
- `/dev/termination-log` takes precedence over the log fallback, so an application that writes
  there yields its own message instead of its log tail. Untested.
- OOM kills and probe failures produce no useful message, so they cannot be attributed. A pod that
  only ever OOMs is detected as broken but never reported.
- Only Java, Python, Node.js and .NET are stamped, so only those languages can be reported on.
- A single operator replica is assumed; the manager does not enable leader election.

## Helpful tools
1. This package uses [kubebuilder markers](https://book.kubebuilder.io/reference/markers.html) to generate kubernetes configs. Run `make manifests` to create crds and roles in `config/crd` and `config/rbac`
2. Generate deepcopy.go by running `make generate`


## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
