# syntax=docker/dockerfile:1
FROM golang:1.18.6 as builder

WORKDIR /build

# build args
ARG MA_VERSION
ARG MA_PROTOCOL

# Note: git clone is not required when building locally.
# RUN git clone --depth=1 --branch ${MA_VERSION} --single-branch https://github.com/iogakos/agent.git

# Note: uncomment to build from local source.
COPY . ./agent

WORKDIR /build/agent
RUN make build-${MA_PROTOCOL}-dbg

FROM bitnami/minideb:bullseye

# build-args
ARG MA_PROTOCOL
ARG MA_OS_ARCH

# Enable TLS
RUN apt-get update && \
    apt-get install -y ca-certificates openssl

# Create metrikad and configure permissions for agent directories.
# This user is required when agent is run by a non-root user, for example
# when the host docker daemon is accessed by a docker proxy.
RUN adduser --system --home /opt/metrikad --disabled-password metrikad
RUN mkdir /etc/metrikad
RUN chown -R metrikad: /etc/metrikad /opt/metrikad

# Copy agent binary and configuration
COPY --from=builder "/build/agent/metrikad-${MA_PROTOCOL}-linux-${MA_OS_ARCH}" "/opt/metrikad/metrikad-${MA_PROTOCOL}"
COPY --from=builder /build/agent/configs /etc/metrikad/configs

# remove setuid and setgid permission from binaries
RUN find / -perm /6000 -type f -exec chmod a-s {} \; 2>/dev/null || true

WORKDIR /opt/metrikad
RUN ln -s metrikad-${MA_PROTOCOL} metrikad
CMD ./metrikad
