# Build driver go binary
FROM golang:1.20.5 as driver-builder

ARG STAGINGVERSION

WORKDIR /daos-csi-driver
ADD . .
RUN make driver BINDIR=/bin

# Start from Kubernetes Debian base.
FROM gke.gcr.io/debian-base:bullseye-v1.4.3-gke.5 as debian
# Install necessary dependencies
RUN clean-install mount bash

# go/gke-releasing-policies#base-images
# We use `gcr.io/distroless/base` because it includes glibc.
FROM gcr.io/distroless/base-debian11 as distroless-base

# The distroless amd64 image has a target triplet of x86_64
FROM distroless-base AS distroless-amd64
ENV LIB_DIR_PREFIX x86_64

# The distroless arm64 image has a target triplet of aarch64
FROM distroless-base AS distroless-arm64
ENV LIB_DIR_PREFIX aarch64

FROM distroless-$TARGETARCH as output-image

# Copy the mount/umount binaries
COPY --from=debian /bin/mount /bin/mount
COPY --from=debian /bin/umount /bin/umount

# Copy shared libraries into distroless base.
COPY --from=debian /lib/${LIB_DIR_PREFIX}-linux-gnu/libselinux.so.1 /lib/${LIB_DIR_PREFIX}-linux-gnu/

COPY --from=debian /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libblkid.so.1 \
                   /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libmount.so.1 \
                   /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libpcre2-8.so.0 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/

# Build stage used for validation of the output-image
FROM output-image as validation-image
COPY --from=debian /lib/${LIB_DIR_PREFIX}-linux-gnu/libtinfo.so.6 \
                   /lib/${LIB_DIR_PREFIX}-linux-gnu/libpcre.so.3 /lib/${LIB_DIR_PREFIX}-linux-gnu/
COPY --from=debian /bin/bash /bin/bash
COPY --from=debian /bin/grep /bin/grep
COPY --from=debian /usr/bin/ldd /usr/bin/ldd
SHELL ["/bin/bash", "-c"]
RUN if ldd /bin/mount | grep "not found"; then echo "!!! Missing deps for mount command !!!" && exit 1; fi
RUN if ldd /bin/umount | grep "not found"; then echo "!!! Missing deps for umount command !!!" && exit 1; fi

# Final build stage, create the real Docker image with ENTRYPOINT
FROM output-image

COPY --from=driver-builder /bin/daos-csi-driver /daos-csi-driver

ENTRYPOINT ["/daos-csi-driver"]