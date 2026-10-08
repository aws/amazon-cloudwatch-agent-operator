// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// This file answers one question: was an already-broken pod broken BY US? EvaluatePod decides
// THAT something is broken; AttributeFailure decides WHO.
//
// # The rule
//
// A failure is ours when either of these holds:
//
//  1. Structural. One of the init containers the operator injected failed ON ITS OWN, or cannot
//     pull its image. That container's name, image, command, volume and resource limits are all
//     operator-set, so if we had not injected there would be no such container to fail. No text
//     is parsed.
//
//     The construction argument covers a failure that container CAUSED. It does not cover one
//     INFLICTED ON IT from outside: a pod drained off a node, evicted, preempted,
//     spot-interrupted or deleted mid-rolling-update has its running init container signalled,
//     and the kubelet reports exit 137 (SIGKILL) or 143 (SIGTERM) for a kill auto-instrumentation
//     had no part in. Two guards keep those out, both in this file: a pod with a
//     DeletionTimestamp is never ours, and a signal-induced exit on the init container is not
//     ours either. The exception is an OOM kill, which IS ours, because the memory limit that was
//     exceeded is one the operator sets.
//
//     It does not cover a failure that has since RECOVERED either. An init container whose
//     current state is Terminated with exit code 0 completed successfully, and the kubelet keeps
//     the earlier attempt in LastTerminationState indefinitely. Reading that stale record would
//     let one transient copy failure explain every later crash of the application, forever, so a
//     container that is currently terminated with exit code 0 is skipped outright.
//
//  2. Evidential. A traceback frame in the crash output names a file inside the injected mount
//     path AND its function is exactly "<module>", meaning our payload died while it was being
//     imported, AND that same traceback carries no frame from ADOT's own loader. Nothing of ours
//     had to be called for such an import to happen.
//
// # Why the function name, and not the path
//
// The injected mount path appears in the output of HEALTHY pods, because ADOT's exporter prints
// full tracebacks whenever it cannot reach a collector. Our files therefore appear in two roles,
// and only the function name separates them:
//
//	File "/otel-auto-instrumentation-python/urllib3/__init__.py", line 15, in <module>  <- ours
//	File "/otel-auto-instrumentation-python/requests/sessions.py", line 784, in send    <- normal
//
// Three simpler rules were implemented and all three were rejected against REAL captured output
// (.tmp/poc-tier2/RESULTS.md findings 3 and 5, fixtures in testdata/):
//
//   - "does our path appear anywhere" is a false positive. A real v0.2.0 run that exited 0
//     printed 21 frames naming our path.
//   - "does our path appear in the last traceback block" is also a false positive on that same
//     healthy run, because the exporter's traceback happened to be printed last.
//   - "a <module> frame in our path, wherever it sits" is a false positive too, on a message
//     whose <module> frame was reached THROUGH ADOT's own loader. An instrumentor that fails to
//     import is caught and skipped by that loader, and the application keeps running; the
//     traceback is in the output only because the loader logged it.
//
// # Why a loader frame exonerates us
//
// ADOT's bootstrap cannot be fatal, by construction of CPython and of the loader itself:
//
//   - _load.py's _load_instrumentors catches the ImportError an instrumentor entry point raises,
//     logs the whole traceback with _logger.exception, and carries on with the next entry point.
//     The instrumentor is skipped; nothing propagates.
//   - site.execsitecustomize() wraps the import of sitecustomize and SWALLOWS anything that does
//     propagate out of it, printing "Error in sitecustomize; set PYTHONVERBOSE for traceback".
//     The interpreter then runs the application normally.
//
// So a <module> frame whose traceback also contains a frame from the loader is evidence of a
// failure that was ABSORBED, not one that killed the application. Whatever killed it is something
// else in the same message - possibly another traceback in that very message, which is why the
// exemption is scoped to one traceback. adotLoaderPaths lists the loader's own files.
//
// # The exemption is per TRACEBACK, not per message
//
// One termination message routinely holds several tracebacks, and they are independent events.
// The loader absorbs only the import IT drove, so its frames say nothing about a fatal import
// somewhere else in the same output. The primary failure this guard exists to catch has exactly
// that shape: a partial version mismatch makes an ADOT instrumentor fail to import, the loader
// catches that and logs its traceback, and then the application's own import of the library our
// payload shadows dies of the same mismatch. Two tracebacks in one message, only the second of
// them fatal. Exempting per message exonerates that message outright, which is the single verdict
// this guard must never reach.
//
// Each message is therefore split on lines that, trimmed, are exactly
// "Traceback (most recent call last):", and every block is judged on its own: a block convicts
// when it holds a <module> frame inside the injected mount path AND no loader frame. The first
// convicting block wins.
//
// Judging every block is NOT the rejected "last traceback block" rule. That one picked one block
// by position and ignored the others, which the healthy exit-0 capture defeats outright, because
// the exporter's traceback happened to be printed last. Nor does ordering matter WITHIN a block:
// ADOT's exporter prints from a background thread, so the order in which one block's lines reach
// the message is not ours to control, and the loader frame that excuses a <module> frame can
// arrive after it. The whole block is read before the block is judged.
//
// # The orphan prefix abstains
//
// Whatever precedes the FIRST traceback header is the ORPHAN PREFIX, and it never convicts. The
// kubelet truncates the message at 2048 bytes keeping the TAIL, so a block's header is routinely
// cut away: the real case-7 fixture literally begins mid-word, at "ent call last):". Text in the
// prefix is the remains of a block whose head is gone, and the lines the cut took may be exactly
// the loader frames that excused a surviving <module> frame. Convicting on evidence that is
// demonstrably incomplete is a false positive, so the prefix only ever produces its own reason
// string, ranked below both a conviction and the loader exemption so that a complete block
// elsewhere in the message still decides.
//
// The price is a guilty block whose own header was truncated away: the guard stays silent and the
// application stays broken. That is a FALSE NEGATIVE of exactly the kind the 2048-byte
// tail-retention limit below already documents, and it is the direction to fail in. A missed
// back-out leaves a broken application broken, which the customer can already see; a wrong
// back-out takes telemetry away from a workload that was never ours to touch, and in v1 a
// back-out is permanent.
//
// # Limits that live in this file (spec section 10)
//
//   - Message attribution is PYTHON ONLY in v1. Only Python prepends the payload root to the
//     application's own import path (PYTHONPATH), so only Python can shadow the application's own
//     libraries. Java uses -javaagent in a separate classloader and Node.js a single --require,
//     neither of which shadows; .NET has not been checked. Java, Node.js and .NET rely on the
//     structural signal alone.
//   - A loader frame is matched on the captured frame PATH, never on the message text. The guilty
//     fixtures contain the prose line "Error in sitecustomize; set PYTHONVERBOSE for traceback:",
//     which is site.execsitecustomize() reporting a failure it swallowed - the very failure that
//     then killed the application's own import. A substring search for "sitecustomize" over the
//     raw message would exonerate exactly the cases the guard exists to catch.
//   - An external kill is told apart from an OOM kill by terminated.Reason alone, which the
//     kubelet sets to "OOMKilled" for the latter. An external kill of our init container that the
//     runtime reports with that same reason would be attributed to us; nothing else in the
//     terminated state distinguishes the two.
//   - Only SIGKILL and SIGTERM are read as a kill from outside, so a container stopped by some
//     other signal from outside the pod would still be attributed to us. That is a deliberate
//     trade: the alternative, excluding every exit >= 128, silently drops a crash of our own
//     init container (SIGSEGV 139, SIGABRT 134) and so hides a real failure. See killedFromOutside.
//   - The 2048-byte, tail-retained truncation means more than ~2KB of output printed after a
//     genuine fatal traceback pushes the "<module>" frames out of the message. That is a FALSE
//     NEGATIVE: we miss a real failure and leave the application broken. A chatty application is
//     the risk. The same cut leaving the "<module>" frames but removing the traceback header
//     above them is the same false negative, reached through the orphan prefix's abstention
//     above.
//   - A language whose injected image is CONCLUSIVELY not the operator's currently configured
//     default is not attributed at all. That is what stops a pod left over from a previous
//     rollout from backing out injection the operator has already moved on from, and it also
//     means an image pinned in an Instrumentation CR is never guarded. The comparison is loose
//     and has three outcomes, not two: when it cannot decide - one side pins a digest and the
//     other a tag, or the kubelet has not reported an image yet - the language IS attributed and
//     the reason says the image could not be compared, because the alternative is to stay silent
//     about a failure the evidence otherwise convicts us of. See configuredImageMatch and
//     compareImageRefs.

package instrumentationguard

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// The waiting reasons the kubelet reports when it cannot pull a container's image. health.go
// reads them too: EvaluatePod has to treat a pull failure as broken before AttributeFailure ever
// runs, and both must agree on the same two strings.
const (
	reasonImagePullBackOff = "ImagePullBackOff"
	reasonErrImagePull     = "ErrImagePull"
)

// pythonFrame matches one line of a Python traceback, e.g.
//
//	File "/otel-auto-instrumentation-python/urllib3/__init__.py", line 15, in <module>
//
// after the leading indentation has been trimmed. The line number is captured only so the reason
// can name it; the match itself does not depend on its value.
var pythonFrame = regexp.MustCompile(`^File "([^"]+)", line (\d+), in (.+)$`)

// moduleLoadFunction is the function name Python reports for a frame that is executing a module's
// own body, i.e. a frame from an import rather than from a call.
const moduleLoadFunction = "<module>"

// pythonTracebackHeader is the line CPython prints before the frames of every traceback it
// formats, so it is what separates one traceback from the next inside a single termination
// message. It is matched after trimming, because the header is printed unindented by the
// interpreter but indented by logging handlers that wrap it.
const pythonTracebackHeader = "Traceback (most recent call last):"

// adotLoaderPaths are the files of ADOT's own bootstrap loader, as sub-paths of the injected mount
// path. A frame in one of them means the import that failed was driven by the loader, which
// catches and logs rather than propagates, so the failure was absorbed; see this file's header.
//
//   - auto_instrumentation/ holds sitecustomize.py, __init__.py and _load.py: the entry point
//     CPython imports and the entry-point loading loop.
//   - distro.py is where load_instrumentor calls entry_point.load(), the import that most
//     commonly raises.
var adotLoaderPaths = []string{
	"/opentelemetry/instrumentation/auto_instrumentation/",
	"/opentelemetry/instrumentation/distro.py",
}

// Attribution is AttributeFailure's verdict.
type Attribution struct {
	// Ours reports whether injected auto-instrumentation caused the failure.
	Ours bool
	// Type is the language the evidence actually named, set only when Ours is true. The back-out
	// disables every injected language, so nothing acts on this: the per-language re-enable that
	// needed it is gone. It is kept because it is the one field that says which language was
	// blamed, which is what an operator reading the logs wants to know.
	Type instrumentation.Type
	// Reason is human-readable and ends up in the Event and in the guard record, so it is written
	// for a customer reading `kubectl describe`.
	Reason string
}

// AttributeFailure reports whether injected auto-instrumentation caused this pod's failure, and
// why. It is pure: it reads only its arguments.
//
// configuredImages is the operator's currently configured default image per language, which the
// caller reads from its environment so this function stays pure. A language whose injected image
// is conclusively not that default is not attributed, and one whose image cannot be compared is
// attributed with the ambiguity named in the reason; see configuredImageMatch. A language absent
// from the map, or present with an empty value, is attributed as normal.
//
// It is only meaningful for a pod EvaluatePod has already called broken.
func AttributeFailure(pod corev1.Pod, configuredImages map[instrumentation.Type]string) Attribution {
	if pod.DeletionTimestamp != nil {
		// The pod is being torn down - a rolling update, a drain, an eviction, a preemption, or a
		// customer deleting it by hand. Its containers are being signalled by something that is
		// not auto-instrumentation, so nothing they report is attributable to us. TrimPodForCache
		// keeps this field for exactly this check.
		return Attribution{Reason: "pod is terminating, so its failure is not attributable to auto-instrumentation"}
	}

	injected := instrumentation.InjectedImages(pod)
	if len(injected) == 0 {
		return Attribution{Reason: "pod carries no auto-instrumentation"}
	}

	if attribution := attributeInitContainer(pod, injected, configuredImages); attribution.Ours {
		return attribution
	}

	pythonImage, python := injected[instrumentation.TypePython]
	if !python {
		// Message attribution is Python-only in v1; see the limits in this file's header comment.
		return Attribution{Reason: "no auto-instrumentation init container failed, and message attribution is python-only"}
	}
	configured, match := configuredImageMatch(instrumentation.TypePython, pythonImage, configuredImages)
	if match == imageMatchDifferent {
		return Attribution{Reason: fmt.Sprintf(
			"pod carries python image %s, which is not the operator's configured default %s, so it is from a superseded rollout",
			pythonImage, configured)}
	}
	attribution := attributePythonMessage(pod)
	if match == imageMatchInconclusive {
		attribution.Reason += imageAmbiguity(instrumentation.TypePython, pythonImage, configured)
	}
	return attribution
}

// configuredImageMatch compares a language's injected image with the operator's currently
// configured default for it, and returns that default alongside the outcome. It is what stops a
// pod that outlived the rollout it was created by from deciding anything: such a pod still carries
// that rollout's image, so attributing its failure would disable injection the operator has
// already moved past on evidence that says nothing about what injection does now - and because a
// back-out is permanent, that mistake costs the workload its telemetry until a human notices.
//
// Only a conclusive imageMatchDifferent means superseded. The comparison is loose by design (see
// compareImageRefs: the kubelet's string is not the configured string), and when it cannot reach a
// verdict - one side pins a digest and the other a tag, or the kubelet has not reported an image
// yet - the guard proceeds with attribution and names the ambiguity in the reason, through
// imageAmbiguity.
//
// That direction is deliberate. Skipping on an inconclusive comparison is silence: the application
// stays down, no Event is emitted, and nothing tells the customer the guard looked. Proceeding
// costs at most a bounded, reversible annotation change on a workload that is already broken, and
// it is still gated on the attribution evidence itself - the pod only gets backed out if its crash
// output or one of our init containers convicts us anyway.
//
// A language with no configured default - the environment variable is unset or empty - has nothing
// to compare against and reads as imageMatchSame, so the guard behaves exactly as it did before
// the comparison existed for an operator that configures none.
func configuredImageMatch(instType instrumentation.Type, injectedImage string, configuredImages map[instrumentation.Type]string) (string, imageMatch) {
	configured, ok := configuredImages[instType]
	if !ok || configured == "" {
		return "", imageMatchSame
	}
	return configured, compareImageRefs(injectedImage, configured)
}

// imageAmbiguity is the note appended to an Attribution.Reason when the injected image could not
// be compared with the configured default. It reaches the Event and the guard record, so a
// customer reading either can see that the guard acted without being able to confirm the pod was
// running the current default.
func imageAmbiguity(instType instrumentation.Type, injectedImage, configuredImage string) string {
	if injectedImage == "" {
		return fmt.Sprintf(
			" (the kubelet has not reported an image for %s yet, so it could not be compared with the operator's configured default %s, and this pod may be from a superseded rollout)",
			instType, configuredImage)
	}
	return fmt.Sprintf(
		" (the kubelet reported %s image %s, which could not be compared with the operator's configured default %s, so this pod may be from a superseded rollout)",
		instType, injectedImage, configuredImage)
}

// attributeInitContainer implements the structural path: it looks for one of OUR init containers
// failing on its own. This needs no evidence beyond the container's existence. Its name, image,
// command, volume and resource limits are all operator-set, so had we not injected, there would be
// no such container at all. A customer's own init container never matches, because its name is not
// in the map.
//
// Three things do not count, all of them covered in this file's header: a kill inflicted from
// outside, which killedFromOutside filters; a failure that has since recovered, i.e. a container
// currently terminated with exit code 0; and a language whose injected image is CONCLUSIVELY not
// the operator's configured default, which means a superseded rollout. A language whose image
// could not be compared is still attributed, and the verdict carries that ambiguity in its reason;
// see configuredImageMatch.
func attributeInitContainer(pod corev1.Pod, injected, configuredImages map[instrumentation.Type]string) Attribution {
	byName := make(map[string]instrumentation.Type, len(injected))
	ambiguity := make(map[instrumentation.Type]string, len(injected))
	for instType, image := range injected {
		configured, match := configuredImageMatch(instType, image, configuredImages)
		if match == imageMatchDifferent {
			continue
		}
		if match == imageMatchInconclusive {
			ambiguity[instType] = imageAmbiguity(instType, image, configured)
		}
		if name, ok := instrumentation.InitContainerName(instType); ok {
			byName[name] = instType
		}
	}

	for _, status := range pod.Status.InitContainerStatuses {
		instType, ours := byName[status.Name]
		if !ours {
			continue
		}
		if terminated := status.State.Terminated; terminated != nil && terminated.ExitCode == 0 {
			// It completed successfully. Whatever LastTerminationState still holds is history the
			// container has already recovered from, so it explains nothing about the pod's
			// current failure.
			continue
		}
		for _, terminated := range []*corev1.ContainerStateTerminated{
			status.State.Terminated,
			status.LastTerminationState.Terminated,
		} {
			if terminated == nil || terminated.ExitCode == 0 || killedFromOutside(terminated) {
				continue
			}
			return Attribution{Ours: true, Type: instType, Reason: fmt.Sprintf(
				"auto-instrumentation init container %s (%s) exited with code %d",
				status.Name, instType, terminated.ExitCode) + ambiguity[instType]}
		}
		if waiting := status.State.Waiting; waiting != nil && isImagePullFailure(waiting.Reason) {
			// It is our image reference that will not pull, so the stalled pod is ours too.
			return Attribution{Ours: true, Type: instType, Reason: fmt.Sprintf(
				"auto-instrumentation init container %s (%s) cannot pull image %s (%s)",
				status.Name, instType, status.Image, waiting.Reason) + ambiguity[instType]}
		}
	}
	return Attribution{}
}

// attributePythonMessage implements the evidential path over the crash output the kubelet copied
// into the pod's status, which is there because the stamp mutator set
// terminationMessagePolicy: FallbackToLogsOnError on the injected container.
//
// Each container's message is its own piece of evidence: a loader frame in one container's output
// says nothing about a module-load failure in another's, so the exemption is never applied across
// containers. Within one message it is applied per traceback block; see this file's header.
//
// The three outcomes a message can produce are ranked, and the ranking matters: a conviction
// anywhere beats everything, the loader exemption beats the orphan prefix's abstention, and the
// pass-through count is only reported when no block held a module-load frame at all.
func attributePythonMessage(pod corev1.Pod) Attribution {
	mountPath, ok := instrumentation.InstrMountPath(instrumentation.TypePython)
	if !ok {
		return Attribution{Reason: "python has no auto-instrumentation mount path"}
	}

	passThrough := 0
	absolved := ""
	orphaned := ""
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == naming.Container() {
			// The CloudWatch agent sidecar is not an auto-instrumented application container, so
			// its output says nothing about the ADOT payload.
			continue
		}
		scan := scanPythonFrames(terminationMessage(status), mountPath)
		passThrough += scan.passThrough

		if guilty := scan.firstConvicting(); guilty != nil {
			return Attribution{Ours: true, Type: instrumentation.TypePython, Reason: fmt.Sprintf(
				"container %s: auto-instrumentation payload failed while being imported: %s line %s",
				status.Name, guilty.moduleFrame[1], guilty.moduleFrame[2])}
		}
		if absorbed := scan.firstAbsorbed(); absorbed != nil && absolved == "" {
			// Absorbed by ADOT's own loader, so it cannot be what killed the application. Keep
			// looking: another container may still hold a real module-load failure.
			absolved = fmt.Sprintf(
				"container %s: the auto-instrumentation payload failed while being imported at %s line %s, but the same traceback runs through ADOT's own loader (%s), which logs and continues, so the failure was absorbed",
				status.Name, absorbed.moduleFrame[1], absorbed.moduleFrame[2], absorbed.loaderFrame)
		}
		if scan.orphan.moduleFrame != nil && orphaned == "" {
			orphaned = fmt.Sprintf(
				"container %s: the auto-instrumentation payload appears in a module-load frame at %s line %s, but that frame sits before any traceback header, so the header carrying its context was truncated away and the frame cannot be judged",
				status.Name, scan.orphan.moduleFrame[1], scan.orphan.moduleFrame[2])
		}
	}

	if absolved != "" {
		return Attribution{Reason: absolved}
	}
	if orphaned != "" {
		return Attribution{Reason: orphaned}
	}
	if passThrough > 0 {
		return Attribution{Reason: fmt.Sprintf("our payload appears only in %d call frame(s), not as a module-load failure", passThrough)}
	}
	return Attribution{Reason: "no auto-instrumentation frame in any container's termination message"}
}

// blockScan is what ONE traceback block yields: the first module-load frame in our payload and
// whether that same block also holds a frame from ADOT's loader. A block is read to its end
// before it is judged, because the loader frame that excuses a <module> frame can be printed
// after it.
type blockScan struct {
	// moduleFrame is pythonFrame's submatches for the first "<module>" frame inside the mount
	// path, or nil when this block has none.
	moduleFrame []string
	// loaderFrame is the path of the first frame inside the mount path that belongs to ADOT's own
	// loader, or "" when this block has none.
	loaderFrame string
}

// messageScan is what one container's whole termination message yields.
type messageScan struct {
	// orphan is the text before the first traceback header. It is kept apart from blocks because
	// it can never convict: see this file's header on the orphan prefix.
	orphan blockScan
	// blocks holds one entry per complete traceback block, in the order the headers appeared.
	blocks []blockScan
	// passThrough counts frames inside the mount path, ANYWHERE in the message including the
	// orphan prefix, whose function is anything other than "<module>": our already-imported code
	// being invoked, which is normal on healthy pods. It is a property of the message rather than
	// of a block, because it answers a different question - "did our files only ever get called"
	// - for which block boundaries are irrelevant.
	passThrough int
}

// firstConvicting returns the first block that convicts the payload: a module-load frame in our
// path with no loader frame in the same traceback to excuse it. It returns nil when no block does.
func (s messageScan) firstConvicting() *blockScan {
	for i := range s.blocks {
		if s.blocks[i].moduleFrame != nil && s.blocks[i].loaderFrame == "" {
			return &s.blocks[i]
		}
	}
	return nil
}

// firstAbsorbed returns the first block holding a module-load frame in our path that ADOT's own
// loader absorbed, or nil when no block does. Its verdict only matters once firstConvicting has
// found nothing, since a traceback the loader swallowed cannot explain away a different one that
// killed the application.
func (s messageScan) firstAbsorbed() *blockScan {
	for i := range s.blocks {
		if s.blocks[i].moduleFrame != nil && s.blocks[i].loaderFrame != "" {
			return &s.blocks[i]
		}
	}
	return nil
}

// scanPythonFrames reads one container's termination message once, assigning every frame it finds
// inside the mount path to the traceback block it was printed in.
func scanPythonFrames(message, mountPath string) messageScan {
	// Index 0 is the orphan prefix, which exists whether or not the message opens with a header;
	// every header appends a block, and frames always land in the newest one.
	blocks := make([]blockScan, 1)
	passThrough := 0
	for _, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == pythonTracebackHeader {
			blocks = append(blocks, blockScan{})
			continue
		}
		frame := pythonFrame.FindStringSubmatch(trimmed)
		if frame == nil || !strings.Contains(frame[1], mountPath) {
			continue
		}
		current := &blocks[len(blocks)-1]
		if current.loaderFrame == "" && isADOTLoaderPath(frame[1]) {
			current.loaderFrame = frame[1]
		}
		if frame[3] == moduleLoadFunction {
			if current.moduleFrame == nil {
				current.moduleFrame = frame
			}
			continue
		}
		passThrough++
	}
	return messageScan{orphan: blocks[0], blocks: blocks[1:], passThrough: passThrough}
}

// isADOTLoaderPath reports whether a traceback frame's file is one of ADOT's own loader files. It
// is given the frame's captured path, never the raw message; see this file's header.
func isADOTLoaderPath(path string) bool {
	for _, loaderPath := range adotLoaderPaths {
		if strings.Contains(path, loaderPath) {
			return true
		}
	}
	return false
}

// terminationMessage returns the crash output to examine for a container. lastState is preferred,
// because a crash-looping container's interesting death is the one before the current attempt;
// the current state is the fallback for a container that is terminated and has not restarted yet.
// The rule applied to either is identical.
func terminationMessage(status corev1.ContainerStatus) string {
	if terminated := status.LastTerminationState.Terminated; terminated != nil && terminated.Message != "" {
		return terminated.Message
	}
	if terminated := status.State.Terminated; terminated != nil {
		return terminated.Message
	}
	return ""
}

// A process killed by signal N exits with 128+N. Only the two signals the orchestrator uses to
// stop a container count as a kill from outside:
//
//	SIGKILL (9)  -> 137   eviction, preemption, node drain, spot interruption, grace period expiry
//	SIGTERM (15) -> 143   ordinary deletion, including a rolling update replacing the pod
//
// Any OTHER fatal signal means the container died of its own fault rather than being stopped:
// SIGSEGV (139), SIGABRT (134), SIGBUS (135), SIGILL (132) and SIGFPE (136) are all crashes. Those
// stay attributable, because the binary that crashed is one the operator put in the pod.
const (
	exitSIGKILL = 137
	exitSIGTERM = 143
	signalKILL  = 9
	signalTERM  = 15
)

// killedFromOutside reports whether a terminated state describes our init container being STOPPED
// by the orchestrator, as opposed to failing on its own. The construction argument in this file's
// header covers only the latter: the init container is operator-set, so a failure it causes is
// ours, but a kill inflicted on it from outside is not.
//
// Both tests are needed because the kubelet does not always populate Signal, leaving the exit code
// as the only evidence.
//
// An OOM kill is deliberately NOT treated as outside, even though it also arrives as exit 137. The
// memory limit the container exceeded is one the operator sets on the injected init container, so
// that failure is ours. terminated.Reason is what separates the two cases.
func killedFromOutside(terminated *corev1.ContainerStateTerminated) bool {
	if terminated.Reason == reasonOOMKilled {
		return false
	}
	if terminated.Signal == signalKILL || terminated.Signal == signalTERM {
		return true
	}
	if terminated.Signal != 0 {
		// Some other signal: a crash, so not from outside.
		return false
	}
	return terminated.ExitCode == exitSIGKILL || terminated.ExitCode == exitSIGTERM
}

// isImagePullFailure reports whether a container's waiting reason means the kubelet cannot pull
// its image.
func isImagePullFailure(reason string) bool {
	return reason == reasonImagePullBackOff || reason == reasonErrImagePull
}
