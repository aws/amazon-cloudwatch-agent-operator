# syntax=docker/dockerfile:1.4
FROM --platform=$BUILDPLATFORM golang:1.25.7 AS builder

# Set build arguments early
ARG TARGETARCH
ARG VERSION_PKG
ARG VERSION
ARG VERSION_DATE
ARG AGENT_VERSION
ARG AUTO_INSTRUMENTATION_JAVA_VERSION
ARG AUTO_INSTRUMENTATION_PYTHON_VERSION
ARG AUTO_INSTRUMENTATION_DOTNET_VERSION
ARG AUTO_INSTRUMENTATION_NODEJS_VERSION
ARG DCMG_EXPORTER_VERSION
ARG NEURON_MONITOR_VERSION
ARG TARGET_ALLOCATOR_VERSION

# Set environment variables
ENV GOPROXY="direct" \
    GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=$TARGETARCH \
    GOPRIVATE="" \
    GOSUMDB=on

WORKDIR /workspace

# Download dependencies with cache mount
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy only necessary files
COPY . .

# Build with cache mount
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -trimpath \
    -ldflags="\
    -X ${VERSION_PKG}.version=${VERSION} \
    -X ${VERSION_PKG}.buildDate=${VERSION_DATE} \
    -X ${VERSION_PKG}.agent=${AGENT_VERSION} \
    -X ${VERSION_PKG}.autoInstrumentationJava=${AUTO_INSTRUMENTATION_JAVA_VERSION} \
    -X ${VERSION_PKG}.autoInstrumentationPython=${AUTO_INSTRUMENTATION_PYTHON_VERSION} \
    -X ${VERSION_PKG}.autoInstrumentationDotNet=${AUTO_INSTRUMENTATION_DOTNET_VERSION} \
    -X ${VERSION_PKG}.autoInstrumentationNodeJS=${AUTO_INSTRUMENTATION_NODEJS_VERSION} \
    -X ${VERSION_PKG}.dcgmExporter=${DCMG_EXPORTER_VERSION} \
    -X ${VERSION_PKG}.neuronMonitor=${NEURON_MONITOR_VERSION} \
    -X ${VERSION_PKG}.targetAllocator=${TARGET_ALLOCATOR_VERSION}" \
    -o manager main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]