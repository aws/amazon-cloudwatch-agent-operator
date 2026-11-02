# Get CA certificates from the Alpine package repo
FROM alpine:3.24 as certificates

RUN apk --no-cache add ca-certificates openssl

# Distrust the roots behind the eMudhra (emSign) intermediates compromised in October 2026
# (https://blog.mozilla.org/security/2026/10/20/protecting-firefox-users-following-the-compromise-of-emudhra-intermediate-certificates/).
# Roots are matched by SPKI SHA-256 and dropped from the bundle the image ships, and the build
# fails if any of them is still in it.
#   bd153ed7... AAA Certificate Services
#   c619e478... emSign Root TLS CA - G3
#   8d417db2... emSign ECC Root CA - G3
RUN set -euo pipefail; \
    distrusted="bd153ed7b0434f6886b17bce8bbe84ed340c7132d702a8f4fa318f756ecbd6f3 \
                c619e4780ed8e0abeda24ad732567de4383a7ea941338f6d1a7d1b49ac792fe9 \
                8d417db2dd8bf5e3084d1e3f196d583849d81bdd4c00c70b9d39369e96b8c782"; \
    work=$(mktemp -d); \
    split_bundle() { \
        rm -f "$work"/cert-*.pem; \
        awk -v dir="$work" '/-----BEGIN CERTIFICATE-----/ { if (out) close(out); out = sprintf("%s/cert-%03d.pem", dir, ++n) } out { print > out }' "$1"; \
    }; \
    is_distrusted() { \
        spki=$(openssl x509 -in "$1" -noout -pubkey | openssl pkey -pubin -outform der | sha256sum | cut -d' ' -f1); \
        case " $distrusted " in *" $spki "*) return 0 ;; esac; \
        return 1; \
    }; \
    split_bundle /etc/ssl/certs/ca-certificates.crt; \
    : > /tmp/ca-certificates.crt; \
    for cert in "$work"/cert-*.pem; do \
        if ! is_distrusted "$cert"; then cat "$cert" >> /tmp/ca-certificates.crt; fi; \
    done; \
    split_bundle /tmp/ca-certificates.crt; \
    for cert in "$work"/cert-*.pem; do \
        if is_distrusted "$cert"; then echo "distrusted root CA still in /tmp/ca-certificates.crt" >&2; exit 1; fi; \
    done; \
    rm -rf "$work"

# Start a new stage from scratch
FROM scratch

ARG TARGETARCH

WORKDIR /root/

# Copy the certs
COPY --from=certificates /tmp/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary built on the host
COPY bin/targetallocator_${TARGETARCH} ./main

ENTRYPOINT ["./main"]
