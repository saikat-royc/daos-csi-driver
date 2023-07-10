# Sample run command:
# GCP_DAOS_CSI_STAGING_VERSION=v1 PROJECT=<your-project>  make build-sidecar-image-and-push
# GCP_DAOS_CSI_STAGING_VERSION=v1 PROJECT=<your-project>  make build-driver-image-and-push
# GCP_DAOS_CSI_STAGING_VERSION=v1 PROJECT=<your-project> OVERLAY=dev make install

# export REGISTRY ?= 
export OVERLAY ?= dev
DRIVER_BINARY=daos-csi-driver
DRIVER_SIDECAR_BINARY=daos-sidecar
# DRIVER_IMAGE = ${REGISTRY}/${DRIVER_BINARY}
STAGINGVERSION=${GCP_DAOS_CSI_STAGING_VERSION}
$(info STAGINGVERSION is $(STAGINGVERSION))

DRIVER_IMAGE = gcr.io/$(PROJECT)/$(DRIVER_BINARY)
SIDECAR_IMAGE=gcr.io/$(PROJECT)/$(DRIVER_SIDECAR_BINARY)
$(info SIDECAR_IMAGE is $(SIDECAR_IMAGE))
$(info DRIVER_IMAGE is $(DRIVER_IMAGE))

BINDIR?=bin

all: driver

build-sidecar-image-and-push: init-buildx
		{                                                                   \
		set -e ;                                                            \
		docker buildx build \
			--platform linux/amd64 \
			--build-arg STAGINGVERSION=$(STAGINGVERSION) \
			--build-arg BUILDPLATFORM=linux/amd64 \
			--build-arg TARGETPLATFORM=linux/amd64 \
			-f ./cmd/sidecar/Dockerfile \
			-t $(SIDECAR_IMAGE):$(STAGINGVERSION) --push .; \
		}

build-driver-image-and-push: init-buildx
		{                                                                   \
		set -e ;                                                            \
		docker buildx build \
			--platform linux/amd64 \
			--build-arg STAGINGVERSION=$(STAGINGVERSION) \
			--build-arg BUILDPLATFORM=linux/amd64 \
			--build-arg TARGETPLATFORM=linux/amd64 \
			-f ./cmd/csi_driver/Dockerfile \
			-t $(DRIVER_IMAGE):$(STAGINGVERSION) --push .; \
		}

sidecar:
	mkdir -p ${BINDIR}
	{                                                                                                                                 \
	set -e ;                                                                                                                          \
		CGO_ENABLED=0 go build -mod=vendor -a -ldflags '-X main.version=$(STAGINGVERSION) -extldflags "-static"' -o ${BINDIR}/${DRIVER_SIDECAR_BINARY} ./cmd/sidecar/main.go; \
		break;                                                                                                                          \
	}

driver:
	mkdir -p ${BINDIR}
	CGO_ENABLED=0 GOOS=linux GOARCH=$(shell dpkg --print-architecture) go build -mod vendor -ldflags "${LDFLAGS}" -o ${BINDIR}/${DRIVER_BINARY} cmd/csi_driver/main.go

install:
	make generate-spec-yaml OVERLAY=${OVERLAY} STAGINGVERSION=${STAGINGVERSION}
	kubectl apply -f ${BINDIR}/daos-csi-driver-specs-generated.yaml

uninstall:
	kubectl delete -k deploy/overlays/${OVERLAY} --wait

generate-spec-yaml:
	mkdir -p ${BINDIR}
	./deploy/install-kustomize.sh
	cd ./deploy/overlays/${OVERLAY}; ../../../${BINDIR}/kustomize edit set image gke.gcr.io/daos-csi-driver=${DRIVER_IMAGE}:${STAGINGVERSION};
	echo "[{\"op\": \"replace\",\"path\": \"/spec/tokenRequests/0/audience\",\"value\": \"${PROJECT}.svc.id.goog\"}]" > ./deploy/overlays/${OVERLAY}/project_patch_csi_driver.json
	kubectl kustomize deploy/overlays/${OVERLAY} | tee ${BINDIR}/daos-csi-driver-specs-generated.yaml > /dev/null
	git restore ./deploy/overlays/${OVERLAY}/kustomization.yaml
	git restore ./deploy/overlays/${OVERLAY}/project_patch_csi_driver.json

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet
