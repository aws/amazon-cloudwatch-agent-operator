# Get CA certificates from the Alpine package repo
FROM alpine:3.18 as certificates

RUN apk --no-cache add ca-certificates

# Start a new stage from scratch
FROM golang:1.21-alpine

# Install Delve debugger
#RUN apk add --no-cache git \
#    && go install github.com/go-delve/delve/cmd/dlv@latest
ARG TARGETARCH

WORKDIR /root/

# Copy the certs
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary built on the host
COPY bin/targetallocator_${TARGETARCH} ./main
EXPOSE 8443
#EXPOSE 2345
#CMD ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "./main"]
ENTRYPOINT ["./main"]
