# Get CA certificates from the latest Ubuntu
FROM alpine:3.21 as certificates

# Install cert and binaries
FROM $CERT_IMAGE as certificates

RUN apk --no-cache add ca-certificates

# Start a new stage from scratch
FROM scratch

ARG TARGETARCH

WORKDIR /root/

# Copy the certs
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary built on the host
COPY bin/targetallocator_${TARGETARCH} ./main

USER 65532:65532

ENTRYPOINT ["./main"]
