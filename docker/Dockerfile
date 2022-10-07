# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.18.6 as builder

WORKDIR /build

# build args
ARG MA_VERSION
ARG MA_PROTOCOL
ARG TARGETARCH
ARG BUILDOS

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

RUN ln -s /usr/bin/dpkg-split /usr/sbin/dpkg-split
RUN ln -s /usr/bin/dpkg-deb /usr/sbin/dpkg-deb
RUN ln -s /bin/rm /usr/sbin/rm
RUN ln -s /bin/tar /usr/sbin/tar
RUN ln -s /bin/nscd /usr/sbin/nscd
RUN ln -s /bin/sss_cache /usr/sbin/sss_cache

# Enable TLS
RUN apt-get update && \
    apt-get install -y ca-certificates openssl

RUN if [ "$TARGETARCH" = "arm64" ] ; then \
    dpkg --add-architecture arm64 && \
    apt-get install -y pkg-config && \
    dpkg-configure -a && \
    apt-get install -y qemu-user-static nscd sssd-tools;\
fi

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
