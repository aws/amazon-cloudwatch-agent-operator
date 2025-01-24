# Get CA certificates from the Alpine package repo
FROM alpine:3.18 as certificates

# Fix apk repo using https https://github.com/alpinelinux/docker-alpine/issues/98#issuecomment-2572849159
RUN sed -i 's|https://|http://|' /etc/apk/repositories

RUN apk --no-cache add ca-certificates

# Start a new stage from scratch
FROM scratch

ARG TARGETARCH

WORKDIR /root/

# Copy the certs
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary built on the host
COPY bin/targetallocator_${TARGETARCH} ./main

ENTRYPOINT ["./main"]
