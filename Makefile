# Sample run command: GCP_DAOS_CSI_STAGING_VERSION=v1 PROJECT=<your-project>  make build-image-and-push
DRIVERBINARY=daos-csi-driver
DRIVER_SIDECAR_BINARY=daos-sidecar
STAGINGVERSION=${GCP_DAOS_CSI_STAGING_VERSION}
$(info STAGINGVERSION is $(STAGINGVERSION))

STAGINGIMAGE=gcr.io/$(PROJECT)/daos-sidecar
$(info STAGINGIMAGE is $(STAGINGIMAGE))

BINDIR?=bin

all: driver

build-image-and-push: init-buildx
		{                                                                   \
		set -e ;                                                            \
		docker buildx build \
			--platform linux/amd64 \
			--build-arg STAGINGVERSION=$(STAGINGVERSION) \
			--build-arg BUILDPLATFORM=linux/amd64 \
			--build-arg TARGETPLATFORM=linux/amd64 \
			-f ./cmd/Dockerfile \
			-t $(STAGINGIMAGE):$(STAGINGVERSION) --push .; \
		}

sidecar:
	mkdir -p ${BINDIR}
	{                                                                                                                                 \
	set -e ;                                                                                                                          \
		CGO_ENABLED=0 go build -mod=vendor -a -ldflags '-X main.version=$(STAGINGVERSION) -extldflags "-static"' -o ${BINDIR}/${DRIVER_SIDECAR_BINARY} ./cmd/; \
		break;                                                                                                                          \
	}

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet
