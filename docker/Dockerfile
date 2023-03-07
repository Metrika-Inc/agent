# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.18.6 as builder-amd64
ENV CC "gcc"

FROM --platform=$BUILDPLATFORM golang:1.18.6 as builder-arm64
ENV CC "aarch64-linux-gnu-gcc"
ENV CGO_ENABLED "1"

# journald
RUN apt-get update
RUN apt-get -y install gcc-aarch64-linux-gnu

FROM builder-$TARGETARCH as builder

# build args
ARG MA_VERSION
ARG MA_PROTOCOL
ARG TARGETARCH
ARG BUILDOS

# journald
RUN apt-get update
RUN apt-get -y install libsystemd-dev

WORKDIR /build

# Note: git clone is not required when building locally.
RUN git clone --depth=1 --branch ${MA_VERSION} --single-branch https://github.com/Metrika-Inc/agent.git

# Note: uncomment to build from local source.
# COPY . ./agent

WORKDIR /build/agent
RUN make build-$BUILDOS-$TARGETARCH

FROM --platform=$BUILDPLATFORM bitnami/minideb:bullseye

# build-args
ARG MA_PROTOCOL
ARG TARGETARCH
ARG BUILDOS

# Enable TLS
RUN apt-get update && \
    apt-get install -y ca-certificates openssl libsystemd-dev

# Create metrikad and configure permissions for agent directories.
# This user is required when agent is run by a non-root user, for example
# when the host docker daemon is accessed by a docker proxy.
RUN adduser --system --home /opt/metrikad --disabled-password metrikad
RUN mkdir -p /etc/metrikad/configs
RUN mkdir -p /opt/metrikad/.cache
RUN chown -R metrikad: /etc/metrikad /opt/metrikad
RUN chmod u+rw /etc/metrikad/configs /opt/metrikad/.cache

# Copy agent binary and configuration
COPY --from=builder "/build/agent/metrikad-$MA_PROTOCOL-$BUILDOS-$TARGETARCH" "/opt/metrikad/metrikad-$MA_PROTOCOL"
COPY --from=builder /build/agent/configs /etc/metrikad/configs

# remove setuid and setgid permission from binaries
RUN find / -perm /6000 -type f -exec chmod a-s {} \; 2>/dev/null || true

WORKDIR /opt/metrikad
RUN ln -s metrikad-${MA_PROTOCOL} metrikad
CMD ./metrikad
