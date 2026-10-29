// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// The fixtures under testdata are REAL terminated.message bytes captured from pods running the
// real ADOT image, not hand-written strings. Both of the attribution rules this one replaced
// passed invented fixtures and then failed on this output, so the three real cases below must be
// driven by these files and nothing else.
const (
	// fixtureGuilty is case 6: ADOT v0.21.0 on python:3.9, where the application's own
	// `import urllib3` resolved to the injected payload and died while importing it.
	fixtureGuilty = "msg-poc-case6-real-guilty-py39.txt"
	// fixtureInnocent is case 7: the same payload on python:3.11, where it loaded fine and
	// instrumented `requests`. The application then dialled a dead port by its own choice, so our
	// files appear as 8 call frames INSIDE the fatal traceback. Its only <module> frame is the
	// application's own `File "<string>", line 2`. Exactly 2048 bytes, and it begins mid-word at
	// "ent call last):" because the kubelet truncated the head away.
	fixtureInnocent = "msg-poc-case7-real-innocent-py311.txt"
	// fixtureHealthy is a real v0.2.0 run that printed 21 frames naming our path and exited 0.
	// This is the capture that kills both the "path appears anywhere" and "last traceback block"
	// rules.
	fixtureHealthy = "msg-real-v0.2.0-healthy-exit0.txt"
	// fixtureTruncatedGuilty is the guilty case AFTER truncation: the case-7 fixture proves the
	// rule survives truncation on an INNOCENT capture, and this one proves the guilty frames are
	// still found when truncation is what delivered them. Derived from the two real captures by
	// the same 2048-byte tail retention the kubelet applies, and committed rather than built at
	// test time so the asserted bytes are fixed:
	//
	//	cat msg-real-v0.2.0-healthy-exit0.txt msg-poc-case6-real-guilty-py39.txt > /tmp/m
	//	head -c 700 msg-real-v0.2.0-healthy-exit0.txt >> /tmp/m
	//	tail -c 2048 /tmp/m > msg-truncated-guilty-tail-py39.txt
	//
	// Everything before the cut is gone, so the message opens mid-traceback on a frame line whose
	// indentation was sliced off, and the guilty <module> frames sit in the middle of the retained
	// tail: preceded by exporter pass-through frames that survived the cut, followed by ~700 bytes
	// of exporter noise that would hide them from any "last traceback block" rule.
	fixtureTruncatedGuilty = "msg-truncated-guilty-tail-py39.txt"
	// fixtureInstrumentorImportError is the false positive the loader-frame rule exists to kill.
	// An ADOT instrumentor failed to import, _load.py caught the ImportError, logged the whole
	// traceback and skipped that instrumentor, and the application then died of its own bug. The
	// logged traceback ends on a <module> frame inside the injected mount path, so the rule
	// without the loader exemption convicts us of a crash we had no part in.
	fixtureInstrumentorImportError = "msg-instrumentor-importerror-innocent.txt"
	// fixtureInstrumentorSyntaxError is the same shape with a SyntaxError instead. It is the one
	// fixture that passed BEFORE the loader-frame rule, and only by accident: CPython prints a
	// SyntaxError's own frame without the ", in <function>" part, so pythonFrame does not match
	// that line at all and there is no <module> frame left to convict. It is kept because the
	// accident is fragile - any Python version that prints the function name would flip it - and
	// the loader-frame rule is what makes it pass on purpose.
	fixtureInstrumentorSyntaxError = "msg-instrumentor-syntaxerror-innocent.txt"
)

// newerPythonImage is defined in pod_reconciler_test.go: a configured default the pod's injected
// image has fallen behind.

func readFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(content)
}

// pythonPod builds a stamped python pod whose application container died with the given crash
// output in lastState, which is the shape the kubelet reports for a crash-looping container.
func pythonPod(message string) corev1.Pod {
	return podWithTerminationMessage(
		map[instrumentation.Type]string{instrumentation.TypePython: testImagePython},
		testAppContainer, message)
}

func podWithTerminationMessage(images map[instrumentation.Type]string, container, message string) corev1.Pod {
	pod := stampedPod("orders-7d9f-abc", images)
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: container,
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			ExitCode: 1,
			Message:  message,
		}},
	}}
	return pod
}

// pythonPodWithInitStatus replaces the python init container status, keeping the name and image
// the stamp put there so the pod still reads as injected.
func pythonPodWithInitStatus(t *testing.T, state corev1.ContainerState) corev1.Pod {
	t.Helper()
	initContainer, ok := instrumentation.InitContainerName(instrumentation.TypePython)
	require.True(t, ok)

	pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name:  initContainer,
		Image: testImagePython,
		State: state,
	}}
	return pod
}

func TestAttributeFailure(t *testing.T) {
	guilty := readFixture(t, fixtureGuilty)
	innocent := readFixture(t, fixtureInnocent)
	healthy := readFixture(t, fixtureHealthy)
	truncatedGuilty := readFixture(t, fixtureTruncatedGuilty)
	require.Len(t, truncatedGuilty, 2048, "the fixture must be the kubelet's truncation length")
	require.True(t, strings.HasPrefix(truncatedGuilty, `File "`),
		"the fixture must open mid-traceback, on a frame line the cut dedented")

	instrumentorImportError := readFixture(t, fixtureInstrumentorImportError)
	instrumentorSyntaxError := readFixture(t, fixtureInstrumentorSyntaxError)

	// combined is the PRIMARY scenario this whole guard exists for, and no single capture has its
	// shape: a partial version mismatch breaks an ADOT instrumentor's import, which the loader
	// catches and logs, and then breaks the application's own import of the library the payload
	// shadows, which nothing catches. One termination message, two tracebacks, only the second of
	// them fatal.
	//
	// The CONCATENATION is synthetic; both halves are genuine captures and neither is edited. The
	// guilty half is spliced in from its first traceback header so that its own two prose lines -
	// the "Error in sitecustomize" report - do not land in the middle of the innocent half's last
	// traceback, where they say nothing. No fixture file is added for this: a hand-written one
	// would be exactly the invented input two earlier attribution rules passed before failing on
	// real bytes.
	combined := instrumentorImportError + guilty[strings.Index(guilty, pythonTracebackHeader):]
	require.Greater(t, len(combined), 2048,
		"the combined message must exceed the kubelet's truncation length for the tail row below to mean anything")
	// combinedTail is what the kubelet would actually store. The cut lands past the orphan prefix
	// AND past the loader block's own header, which demotes that block to the orphan prefix, so
	// the loader exemption no longer applies to anything and the guilty block decides alone.
	combinedTail := combined[len(combined)-2048:]
	require.Len(t, combinedTail, 2048, "the tail must be the kubelet's truncation length")
	require.False(t, strings.HasPrefix(combinedTail, pythonTracebackHeader),
		"the cut must land inside the first block, not on its header - re-capture the fixtures and re-check the rows below if this fails")

	for _, tt := range []struct {
		name string
		pod  func(t *testing.T) corev1.Pod
		// configured is the operator's configured default image per language. Left nil by almost
		// every row, which is the "no configured default" case: the superseded-image comparison
		// does not apply and attribution behaves as it would with the comparison absent.
		configured   map[instrumentation.Type]string
		wantOurs     bool
		wantType     instrumentation.Type
		wantContains string
	}{
		{
			name:         "real capture: our payload died while being imported",
			pod:          func(*testing.T) corev1.Pod { return pythonPod(guilty) },
			wantOurs:     true,
			wantType:     instrumentation.TypePython,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name:         "real capture: our files are only call frames in the fatal traceback",
			pod:          func(*testing.T) corev1.Pod { return pythonPod(innocent) },
			wantContains: "our payload appears only in 8 call frame(s)",
		},
		{
			name:         "real capture: 21 call frames from a run that exited 0",
			pod:          func(*testing.T) corev1.Pod { return pythonPod(healthy) },
			wantContains: "our payload appears only in 21 call frame(s)",
		},
		{
			name: "interleaved: our fatal traceback followed by exporter noise",
			// A guilty block convicts wherever in the message it sits, and nothing printed after
			// it waters it down. The ADOT exporter prints from a background thread, so this
			// ordering is routine.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(guilty + innocent) },
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "truncated to its last 2048 bytes with the guilty frames in the retained tail",
			// The counterpart to the innocent case-7 capture: there, truncation must not produce a
			// false positive; here, it must not hide a real failure.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(truncatedGuilty) },
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "real shape: an instrumentor's ImportError was caught and logged by ADOT's loader",
			// The <module> frame is in our mount path, so the rule without the loader exemption
			// convicts. _load.py caught it, logged it and skipped that instrumentor, and the
			// application died of its own RuntimeError.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(instrumentorImportError) },
			wantContains: "runs through ADOT's own loader",
		},
		{
			name: "real shape: an instrumentor's SyntaxError was caught and logged by ADOT's loader",
			// Passes for the accidental reason documented on fixtureInstrumentorSyntaxError - the
			// SyntaxError frame line carries no function name, so pythonFrame skips it - and for
			// the deliberate one, the loader frames in the same message. With no <module> frame
			// left to judge, the loader's own frames are counted as the pass-through call frames
			// they are, and the verdict reads like any other healthy capture.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(instrumentorSyntaxError) },
			wantContains: "our payload appears only in 3 call frame(s)",
		},
		{
			name: "an absorbed instrumentor failure AND the application's own fatal import, in one message",
			// The loader block holds a <module> frame in our path and is excused; the guilty block
			// holds one and is not. Exempting per MESSAGE instead of per block exonerates this
			// message outright, which is the false negative that motivated the per-block scope.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(combined) },
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "the same message truncated to the last 2048 bytes the kubelet keeps",
			// The cut takes the loader block's header with it, so that block becomes the orphan
			// prefix and abstains instead of absolving. The guilty block still has its header and
			// still convicts.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(combinedTail) },
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "a module frame with no traceback header above it at all",
			// Ordering within a block does not decide anything - the loader frame that excuses a
			// <module> frame is printed after it here - but this hand-written message has no
			// header, so the whole of it is the orphan prefix and the abstention comes from the
			// missing header rather than from the loader. Truncation keeps the tail, so the lines
			// a cut removed could have been the loader frames that excuse this frame; the guard
			// will not convict on evidence it can prove is incomplete.
			pod: func(*testing.T) corev1.Pod {
				return pythonPod(
					"File \"/otel-auto-instrumentation-python/opentelemetry/instrumentation/requests/__init__.py\", line 174, in <module>\n" +
						"    from os import nope\n" +
						"ImportError: cannot import name 'nope'\n" +
						"  File \"/otel-auto-instrumentation-python/opentelemetry/instrumentation/auto_instrumentation/_load.py\", line 109, in _load_instrumentors\n" +
						"    distro.load_instrumentor(entry_point, skip_dep_check=True)\n")
			},
			wantContains: "sits before any traceback header",
		},
		{
			name: "a loader frame in one container does not excuse another container's failure",
			// Each message is its own piece of evidence.
			pod: func(*testing.T) corev1.Pod {
				pod := pythonPod(instrumentorImportError)
				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
					Name: "worker",
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Message:  guilty,
					}},
				})
				return pod
			},
			wantOurs:     true,
			wantContains: "container worker: auto-instrumentation payload failed while being imported",
		},
		{
			name:         "empty message",
			pod:          func(*testing.T) corev1.Pod { return pythonPod("") },
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "no termination message at all",
			pod: func(*testing.T) corev1.Pod {
				pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name:  testAppContainer,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				}}
				return pod
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "the message is read from state.terminated when lastState has none",
			pod: func(*testing.T) corev1.Pod {
				pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name:  testAppContainer,
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Message: guilty}},
				}}
				return pod
			},
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "our init container exited non-zero",
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				})
			},
			wantOurs:     true,
			wantContains: "exited with code 1",
		},
		{
			name: "our init container exited non-zero before the last restart",
			pod: func(t *testing.T) corev1.Pod {
				pod := pythonPodWithInitStatus(t, corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				})
				pod.Status.InitContainerStatuses[0].LastTerminationState = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 2},
				}
				return pod
			},
			wantOurs:     true,
			wantContains: "exited with code 2",
		},
		{
			name: "our init container cannot pull its image (ImagePullBackOff)",
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: reasonImagePullBackOff},
				})
			},
			wantOurs:     true,
			wantContains: "cannot pull image " + testImagePython + " (ImagePullBackOff)",
		},
		{
			name: "our init container cannot pull its image (ErrImagePull)",
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: reasonErrImagePull},
				})
			},
			wantOurs:     true,
			wantContains: "cannot pull image " + testImagePython + " (ErrImagePull)",
		},
		{
			name: "our init container exited 0",
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				})
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "our init container failed once and then succeeded",
			// The kubelet keeps the failed attempt in LastTerminationState for the life of the
			// pod. Counting it would let one transient copy failure explain every later crash of
			// the application, and would roll back a workload that is running perfectly well.
			pod: func(t *testing.T) corev1.Pod {
				pod := pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				})
				pod.Status.InitContainerStatuses[0].RestartCount = 1
				pod.Status.InitContainerStatuses[0].LastTerminationState = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				}
				return pod
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "our init container was SIGKILLed from outside (node drain, eviction, preemption)",
			// Exit 137 is 128+SIGKILL. The container did not fail; it was killed. Blaming
			// auto-instrumentation here would disable it for the whole workload over a pod kill
			// it had no part in.
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Signal: 9, Reason: "Error"},
				})
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "our init container was SIGTERMed from outside with no Signal reported",
			// The kubelet does not always populate Signal, so the 128+ exit code has to carry the
			// decision on its own. 143 is 128+SIGTERM.
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 143, Reason: "Error"},
				})
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "our init container was killed from outside before the last restart",
			pod: func(t *testing.T) corev1.Pod {
				pod := pythonPodWithInitStatus(t, corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				})
				pod.Status.InitContainerStatuses[0].LastTerminationState = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Signal: 9, Reason: "Error"},
				}
				return pod
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "our init container was OOMKilled",
			// Also exit 137, but OURS: the memory limit it exceeded is one the operator sets.
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Signal: 9, Reason: reasonOOMKilled},
				})
			},
			wantOurs:     true,
			wantContains: "exited with code 137",
		},
		{
			name: "our init container segfaulted, which is a crash and not a kill from outside",
			// 139 is 128+SIGSEGV. Excluding every exit >= 128 would hide this, and a crash of a
			// binary the operator put in the pod is exactly what the guard exists to catch.
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 139, Signal: 11, Reason: "Error"},
				})
			},
			wantOurs:     true,
			wantContains: "exited with code 139",
		},
		{
			name: "our init container aborted with no Signal reported",
			// 134 is 128+SIGABRT, and the kubelet left Signal unset, so the exit code decides.
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 134, Reason: "Error"},
				})
			},
			wantOurs:     true,
			wantContains: "exited with code 134",
		},
		{
			name: "a terminating pod is never ours, however it failed",
			// A pod mid-delete gets its containers signalled by the cluster, not by us. This is
			// the same case the exit-code guard above catches, caught earlier and without
			// depending on what the kubelet wrote into the terminated state.
			pod: func(t *testing.T) corev1.Pod {
				pod := pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				})
				deletedAt := metav1.NewTime(testNow)
				pod.DeletionTimestamp = &deletedAt
				return pod
			},
			wantContains: "pod is terminating",
		},
		{
			name: "a terminating pod whose message would otherwise convict us",
			pod: func(*testing.T) corev1.Pod {
				pod := pythonPod(guilty)
				deletedAt := metav1.NewTime(testNow)
				pod.DeletionTimestamp = &deletedAt
				return pod
			},
			wantContains: "pod is terminating",
		},
		{
			name: "the customer's own init container failed",
			pod: func(*testing.T) corev1.Pod {
				pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
				pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
					Name:  "migrate-db",
					Image: "registry.example.com/migrate:1",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
				})
				return pod
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "the customer's own init container cannot pull its image",
			pod: func(*testing.T) corev1.Pod {
				pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
				pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
					Name:  "migrate-db",
					Image: "registry.example.com/migrate:1",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonImagePullBackOff}},
				})
				return pod
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "a java-only pod carrying a python-shaped message",
			// Message attribution is python-only in v1, so these bytes must not be read.
			pod: func(*testing.T) corev1.Pod {
				return podWithTerminationMessage(
					map[instrumentation.Type]string{instrumentation.TypeJava: testImageJava},
					testAppContainer, guilty)
			},
			wantContains: "message attribution is python-only",
		},
		{
			name: "the message is on the CloudWatch agent sidecar",
			pod: func(*testing.T) corev1.Pod {
				return podWithTerminationMessage(
					map[instrumentation.Type]string{instrumentation.TypePython: testImagePython},
					naming.Container(), guilty)
			},
			wantContains: "no auto-instrumentation frame",
		},
		{
			name: "an unstamped pod",
			pod: func(*testing.T) corev1.Pod {
				pod := pythonPod(guilty)
				delete(pod.Labels, instrumentation.LabelAutoInstrumented)
				return pod
			},
			wantContains: "pod carries no auto-instrumentation",
		},
		{
			name: "a pod left over from a superseded rollout, however guilty its message",
			// The operator has already moved to a newer default, so this pod's failure says
			// nothing about what injection does now - and because a back-out is permanent,
			// attributing it would cost the workload its telemetry until a human intervened.
			pod:          func(*testing.T) corev1.Pod { return pythonPod(guilty) },
			configured:   map[instrumentation.Type]string{instrumentation.TypePython: newerPythonImage},
			wantContains: "superseded rollout",
		},
		{
			name: "a superseded language's init container failure is not ours either",
			pod: func(t *testing.T) corev1.Pod {
				return pythonPodWithInitStatus(t, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				})
			},
			configured:   map[instrumentation.Type]string{instrumentation.TypePython: newerPythonImage},
			wantContains: "superseded rollout",
		},
		{
			name: "a Docker Hub short name the runtime expanded still matches",
			// compareImageRefs is loose on purpose: an exact comparison here would make the guard
			// attribute nothing on a cluster whose runtime reports a different spelling of the
			// image the operator was configured with.
			pod: func(*testing.T) corev1.Pod {
				return podWithTerminationMessage(
					map[instrumentation.Type]string{instrumentation.TypePython: "docker.io/library/python-instrumentation:v1"},
					testAppContainer, guilty)
			},
			configured:   map[instrumentation.Type]string{instrumentation.TypePython: "python-instrumentation:v1"},
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "the configured default resolved to a digest still matches",
			pod:  func(*testing.T) corev1.Pod { return pythonPod(guilty) },
			configured: map[instrumentation.Type]string{
				instrumentation.TypePython: testImagePython + "@sha256:f3b0c9a1",
			},
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "a language with no configured default is attributed as before",
			pod:  func(*testing.T) corev1.Pod { return pythonPod(guilty) },
			// The operator was started without --auto-instrumentation-python-image, so there is
			// nothing to compare against and nothing is skipped.
			configured:   map[instrumentation.Type]string{instrumentation.TypePython: ""},
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "a superseded java pod is skipped, and python still convicts",
			// The skip is per language, like everything else about the image comparison.
			pod: func(*testing.T) corev1.Pod {
				pod := podWithTerminationMessage(map[instrumentation.Type]string{
					instrumentation.TypePython: testImagePython,
					instrumentation.TypeJava:   testImageJava,
				}, testAppContainer, guilty)
				javaInit, ok := instrumentation.InitContainerName(instrumentation.TypeJava)
				require.True(t, ok)
				for i := range pod.Status.InitContainerStatuses {
					if pod.Status.InitContainerStatuses[i].Name == javaInit {
						pod.Status.InitContainerStatuses[i].State = corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
						}
					}
				}
				return pod
			},
			configured: map[instrumentation.Type]string{
				instrumentation.TypePython: testImagePython,
				instrumentation.TypeJava:   "public.ecr.aws/aws-observability/adot-autoinstrumentation-java:v0.0.2",
			},
			wantOurs:     true,
			wantContains: "/otel-auto-instrumentation-python/urllib3/__init__.py line 15",
		},
		{
			name: "a java init container failure names java",
			pod: func(*testing.T) corev1.Pod {
				pod := stampedPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypeJava: testImageJava})
				javaInit, ok := instrumentation.InitContainerName(instrumentation.TypeJava)
				require.True(t, ok)
				pod.Status.InitContainerStatuses[0].Name = javaInit
				pod.Status.InitContainerStatuses[0].State = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				}
				return pod
			},
			wantOurs:     true,
			wantType:     instrumentation.TypeJava,
			wantContains: "exited with code 1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attribution := AttributeFailure(tt.pod(t), tt.configured)
			assert.Equal(t, tt.wantOurs, attribution.Ours, "reason: %s", attribution.Reason)
			assert.Contains(t, attribution.Reason, tt.wantContains)

			if !tt.wantOurs {
				assert.Empty(t, attribution.Type, "an unattributed failure names no language")
				return
			}
			// Every guilty row below is a python pod, so python is the default expectation and
			// wantType only has to be stated where a row blames another language.
			wantType := tt.wantType
			if wantType == "" {
				wantType = instrumentation.TypePython
			}
			assert.Equal(t, wantType, attribution.Type)
		})
	}
}

// TestAttributeFailureAnUncomparableImageIsStillAttributed covers the third outcome of the image
// comparison. Treating "cannot compare" as "superseded" was silence: the guard skipped the pod,
// emitted nothing, and left the application crash-looping with no sign it had looked. Acting
// instead costs a bounded, reversible annotation change on a workload that is already broken, and
// the reason says the image could not be compared so the customer can see why.
func TestAttributeFailureAnUncomparableImageIsStillAttributed(t *testing.T) {
	guilty := readFixture(t, fixtureGuilty)
	// A digest with no tag against a configured default that carries only a tag: the two share
	// nothing but the repository. This is the pair that used to compare EQUAL.
	digestOnly := "public.ecr.aws/aws-observability/adot-autoinstrumentation-python@sha256:f3b0c9a1"
	configured := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}

	t.Run("the message path", func(t *testing.T) {
		attribution := AttributeFailure(podWithTerminationMessage(
			map[instrumentation.Type]string{instrumentation.TypePython: digestOnly},
			testAppContainer, guilty), configured)

		assert.True(t, attribution.Ours)
		assert.Contains(t, attribution.Reason, "/otel-auto-instrumentation-python/urllib3/__init__.py line 15")
		assert.Contains(t, attribution.Reason, "could not be compared with the operator's configured default "+testImagePython)
	})

	t.Run("the init container path", func(t *testing.T) {
		pod := pythonPodWithInitStatus(t, corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
		})
		pod.Status.InitContainerStatuses[0].Image = digestOnly

		attribution := AttributeFailure(pod, configured)

		assert.True(t, attribution.Ours)
		assert.Contains(t, attribution.Reason, "exited with code 1")
		assert.Contains(t, attribution.Reason, "could not be compared with the operator's configured default "+testImagePython)
	})

	t.Run("the kubelet has not reported an image yet", func(t *testing.T) {
		pod := pythonPodWithInitStatus(t, corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
		})
		pod.Status.InitContainerStatuses[0].Image = ""

		attribution := AttributeFailure(pod, configured)

		assert.True(t, attribution.Ours, "an empty image is not evidence of a superseded rollout")
		assert.Contains(t, attribution.Reason, "the kubelet has not reported an image for python yet")
	})

	t.Run("a conclusively different tag is still superseded", func(t *testing.T) {
		// The counterpart: what the comparison CAN decide, it still decides, so the skip this
		// feature narrowed is not the skip it removed.
		attribution := AttributeFailure(pythonPod(guilty),
			map[instrumentation.Type]string{instrumentation.TypePython: newerPythonImage})

		assert.False(t, attribution.Ours)
		assert.Contains(t, attribution.Reason, "superseded rollout")
	})
}

// TestAttributeFailureUsesTheInjectedMountPath pins the matcher to
// instrumentation.InstrMountPath, the accessor over the constant injection itself uses, so the
// matcher cannot drift from the path that is actually mounted.
func TestAttributeFailureUsesTheInjectedMountPath(t *testing.T) {
	mountPath, ok := instrumentation.InstrMountPath(instrumentation.TypePython)
	require.True(t, ok)

	attribution := AttributeFailure(pythonPod(
		"Traceback (most recent call last):\n"+
			"  File \""+mountPath+"/some/module.py\", line 7, in <module>\n"+
			"ImportError: boom\n"), nil)
	assert.True(t, attribution.Ours)
	assert.Contains(t, attribution.Reason, mountPath+"/some/module.py line 7")
}

// TestAttributeFailureLoaderPathsAreUnderTheMountPath pins the loader sub-paths to the injected
// mount path, so a frame in an application's own vendored copy of opentelemetry - which is not
// under the mount path - cannot exonerate the payload. The traceback header is part of the input
// because the exemption is now judged per block and a block is what the header opens; without it
// these frames would be the orphan prefix and the test would be measuring the abstention instead.
func TestAttributeFailureLoaderPathsAreUnderTheMountPath(t *testing.T) {
	mountPath, ok := instrumentation.InstrMountPath(instrumentation.TypePython)
	require.True(t, ok)

	attribution := AttributeFailure(pythonPod(
		pythonTracebackHeader+"\n"+
			"  File \""+mountPath+"/urllib3/__init__.py\", line 15, in <module>\n"+
			"  File \"/app/vendor/opentelemetry/instrumentation/distro.py\", line 50, in load_instrumentor\n"), nil)
	assert.True(t, attribution.Ours,
		"a loader-shaped path outside the injected mount path is the application's own code")
}

// TestLoaderFrameCountsInTheFixtures records the measurement the loader-frame rule rests on: the
// guilty captures contain NO loader frame, and the two instrumentor captures do. It is asserted
// rather than written in a comment because the rule is only sound while that holds. The question
// is about the whole message, so the orphan prefix counts here even though it can never convict:
// the syntax-error capture's loader frames are all in its prefix.
func TestLoaderFrameCountsInTheFixtures(t *testing.T) {
	mountPath, ok := instrumentation.InstrMountPath(instrumentation.TypePython)
	require.True(t, ok)

	for _, tt := range []struct {
		fixture         string
		wantLoaderFrame bool
	}{
		{fixtureGuilty, false},
		{fixtureTruncatedGuilty, false},
		{fixtureInnocent, false},
		{fixtureHealthy, false},
		{fixtureInstrumentorImportError, true},
		{fixtureInstrumentorSyntaxError, true},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			scan := scanPythonFrames(readFixture(t, tt.fixture), mountPath)
			loaderFrame := scan.orphan.loaderFrame
			for _, block := range scan.blocks {
				if loaderFrame == "" {
					loaderFrame = block.loaderFrame
				}
			}
			assert.Equal(t, tt.wantLoaderFrame, loaderFrame != "", "loaderFrame: %q", loaderFrame)
		})
	}
}

// TestScanPythonFramesSplitsOnTheTracebackHeader pins the block boundaries themselves, which the
// verdict tests can only observe indirectly. The message below is the combined shape in miniature:
// an orphan prefix, a loader-excused block, and a guilty one.
func TestScanPythonFramesSplitsOnTheTracebackHeader(t *testing.T) {
	mountPath, ok := instrumentation.InstrMountPath(instrumentation.TypePython)
	require.True(t, ok)

	scan := scanPythonFrames(
		"  File \""+mountPath+"/requests/sessions.py\", line 784, in send\n"+
			pythonTracebackHeader+"\n"+
			"  File \""+mountPath+"/opentelemetry/instrumentation/distro.py\", line 50, in load_instrumentor\n"+
			"  File \""+mountPath+"/opentelemetry/instrumentation/requests/__init__.py\", line 174, in <module>\n"+
			"  "+pythonTracebackHeader+"\n"+
			"  File \""+mountPath+"/urllib3/__init__.py\", line 15, in <module>\n", mountPath)

	require.Len(t, scan.blocks, 2, "one block per header, and the prefix is not one of them")
	assert.Nil(t, scan.orphan.moduleFrame, "the prefix holds only a pass-through call frame")
	// One in the prefix and one in the loader block: the count spans the whole message, which is
	// what keeps the "appears only in N call frame(s)" reasons at the numbers the real captures
	// produced before blocks existed.
	assert.Equal(t, 2, scan.passThrough, "pass-through frames are counted across the whole message")

	require.NotNil(t, scan.firstAbsorbed())
	assert.Equal(t, mountPath+"/opentelemetry/instrumentation/distro.py", scan.firstAbsorbed().loaderFrame)

	// The indented second header still opens a block, and the loader frame in the block above it
	// does not reach into this one.
	guilty := scan.firstConvicting()
	require.NotNil(t, guilty)
	assert.Equal(t, mountPath+"/urllib3/__init__.py", guilty.moduleFrame[1])
	assert.Equal(t, "15", guilty.moduleFrame[2])
}
