# Get CA certificates from the latest Ubuntu
ARG CERT_IMAGE=ubuntu:latest

# Install cert and binaries
FROM $CERT_IMAGE as cert

RUN apt-get update &&  \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Start a new stage from scratch
FROM scratch

ARG TARGETARCH

WORKDIR /root/

# Copy the certs
COPY --from=cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary built on the host
COPY bin/targetallocator_${TARGETARCH} ./main

ENTRYPOINT ["./main"]
