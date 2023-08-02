# Current Operator version
VERSION ?= "0.1"

# Image URL to use all building/pushing image targets
IMG ?= $(CLOUDWATCH_AGENT_OPERATOR_IMAGE)
ARCH ?= $(shell go env GOARCH)

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# by default, do not run the manager with webhooks enabled. This only affects local runs, not the build or in-cluster deployments.
ENABLE_WEBHOOKS ?= false
START_KIND_CLUSTER ?= true

KUBE_VERSION ?= 1.24
KIND_CONFIG ?= kind-$(KUBE_VERSION).yaml
KIND_CLUSTER_NAME ?= "cwa-operator"

OPERATOR_SDK_VERSION ?= 1.29.0

CERTMANAGER_VERSION ?= 1.10.0

ifndef ignore-not-found
  ignore-not-found = false
endif

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## On MacOS, use gsed instead of sed, to make sed behavior
## consistent with Linux.
SED ?= $(shell which gsed 2>/dev/null || which sed)

.PHONY: ensure-generate-is-noop
ensure-generate-is-noop: VERSION=$(OPERATOR_VERSION)
ensure-generate-is-noop: USER=open-telemetry
ensure-generate-is-noop: set-image-controller generate bundle
	@# on make bundle config/manager/kustomization.yaml includes changes, which should be ignored for the below check
	@git restore config/manager/kustomization.yaml
	@git diff -s --exit-code apis/v1alpha1/zz_generated.*.go || (echo "Build failed: a model has been changed but the generated resources aren't up to date. Run 'make generate' and update your PR." && exit 1)
	@git diff -s --exit-code bundle config || (echo "Build failed: the bundle, config files has been changed but the generated bundle, config files aren't up to date. Run 'make bundle' and update your PR." && git diff && exit 1)
	@git diff -s --exit-code bundle.Dockerfile || (echo "Build failed: the bundle.Dockerfile file has been changed. The file should be the same as generated one. Run 'make bundle' and update your PR." && git diff && exit 1)
	@git diff -s --exit-code docs/api.md || (echo "Build failed: the api.md file has been changed but the generated api.md file isn't up to date. Run 'make api-docs' and update your PR." && git diff && exit 1)

.PHONY: all
all: manager
.PHONY: ci
ci: test

# Run tests
# setup-envtest uses KUBEBUILDER_ASSETS which points to a directory with binaries (api-server, etcd and kubectl)
.PHONY: test
test: generate fmt vet ensure-generate-is-noop envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(KUBE_VERSION) -p path)" go test ${GOTEST_OPTS} ./...
	cd cmd/otel-allocator && KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(KUBE_VERSION) -p path)" go test ${GOTEST_OPTS} ./...
	cd cmd/operator-opamp-bridge && KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(KUBE_VERSION) -p path)" go test ${GOTEST_OPTS} ./...

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	ENABLE_WEBHOOKS=$(ENABLE_WEBHOOKS) go run -ldflags ${LD_FLAGS} ./main.go --zap-devel

# Install CRDs into a cluster
.PHONY: install
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

# Set the controller image parameters
.PHONY: set-image-controller
set-image-controller: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}

# Deploy controller in the current Kubernetes context, configured in ~/.kube/config
.PHONY: deploy
deploy: set-image-controller
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Undeploy controller in the current Kubernetes context, configured in ~/.kube/config
.PHONY: undeploy
undeploy: set-image-controller
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

# Generates the released manifests
.PHONY: release-artifacts
release-artifacts: set-image-controller
	$(KUSTOMIZE) build config/default -o scripts/eks/apm/apm.yaml

.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Run go lint against code
.PHONY: lint
lint:
	golangci-lint run
	cd cmd/otel-allocator && golangci-lint run
	cd cmd/operator-opamp-bridge && golangci-lint run

# Generate code
.PHONY: generate
generate: controller-gen api-docs
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the container image, used only for local dev purposes
# buildx is used to ensure same results for arm based systems (m1/2 chips)
.PHONY: container
container:
	docker buildx build --load --platform linux/${ARCH} -t ${IMG} .

# Push the container image, used only for local dev purposes
.PHONY: container-push
container-push:
	docker push ${IMG}

KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
CHLOGGEN ?= $(LOCALBIN)/chloggen

KUSTOMIZE_VERSION ?= v5.0.3
CONTROLLER_TOOLS_VERSION ?= v0.12.0


.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest


# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
go get -d $(2)@$(3) ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

CRDOC = $(shell pwd)/bin/crdoc
.PHONY: crdoc
crdoc: ## Download crdoc locally if necessary.
	$(call go-get-tool,$(CRDOC), fybrik.io/crdoc,v0.5.2)

.PHONY: api-docs
api-docs: crdoc kustomize
	@{ \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ; \
	$(KUSTOMIZE) build config/crd -o $$TMP_DIR/crd-output.yaml ;\
	$(CRDOC) --resources $$TMP_DIR/crd-output.yaml --output docs/api.md ;\
	}


.PHONY: kind
kind:
ifeq (, $(shell which kind))
	@{ \
	set -e ;\
	echo "" ;\
	echo "ERROR: kind not found." ;\
	echo "Please check https://kind.sigs.k8s.io/docs/user/quick-start/#installation for installation instructions and try again." ;\
	echo "" ;\
	exit 1 ;\
	}
else
KIND=$(shell which kind)
endif

OPERATOR_SDK = $(shell pwd)/bin/operator-sdk
.PHONY: operator-sdk
operator-sdk:
	@{ \
	set -e ;\
	if (`pwd`/bin/operator-sdk version | grep ${OPERATOR_SDK_VERSION}) > /dev/null 2>&1 ; then \
		exit 0; \
	fi ;\
	[ -d bin ] || mkdir bin ;\
	curl -L -o $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/v${OPERATOR_SDK_VERSION}/operator-sdk_`go env GOOS`_`go env GOARCH`;\
	chmod +x $(OPERATOR_SDK) ;\
	}
