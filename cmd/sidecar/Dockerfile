ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM golang:1.20.5 as sidecarbuilder

ARG STAGINGVERSION
ARG TARGETPLATFORM

WORKDIR /go/src/sigs.k8s.io/daos-csi-driver
ADD . .
RUN GOARCH=$(echo $TARGETPLATFORM | cut -f2 -d '/') make sidecar BINDIR=/bin GCP_DAOS_CSI_STAGING_VERSION=${STAGINGVERSION}


ARG BASE=rockylinux/rockylinux:8
FROM rockylinux/rockylinux:8 as rockybuilder
RUN dnf -y install git virtualenv epel-release dnf-plugins-core && \
    dnf config-manager --enable powertools && \
    dnf config-manager --save --setopt=assumeyes=True
RUN git clone --recurse-submodules https://github.com/daos-stack/daos.git --branch release/2.4

WORKDIR daos
RUN utils/scripts/install-el8.sh
RUN virtualenv venv
RUN source venv/bin/activate && \
  pip install --upgrade pip && \
  pip install -r requirements.txt
RUN source venv/bin/activate && \
  scons --jobs="$(nproc --all)" --build-deps=yes install PREFIX=/opt/daos

FROM rockylinux/rockylinux:8
RUN dnf -y install epel-release && dnf config-manager --enable powertools
RUN dnf -y install \
  kmod hwloc openssl Lmod ipmctl ndctl numactl protobuf-c \
  json-c glibc-langpack-en e2fsprogs file sg3_utils \
  libevent libunwind libyaml librdmacm libuuid \
  libtool libtool libtool-ltdl libiscsi libipmctl libaio lmdb-libs fuse3

COPY --from=rockybuilder /opt/daos /opt/daos
RUN mkdir -p /var/run/daos_agent
RUN ln -s /opt/daos/prereq/release/spdk/share/spdk /usr/share/spdk
ENV PATH=/opt/daos/bin:$PATH

ARG SIDECAR_BINARY=daos-sidecar
COPY --from=sidecarbuilder /bin/${SIDECAR_BINARY} /${SIDECAR_BINARY}
COPY /cmd/sidecar/startup.sh /startup.sh  
ENTRYPOINT ["/startup.sh"]
