# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= samba-operator-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
TAG ?= latest
IMG ?= quay.io/samba.org/samba-operator:$(TAG)

# Produce CRDs that work on Kubernetes 1.16 or later
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"

CHECK_GOFMT_FLAGS ?= -e -s -l

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CONTAINER_BUILD_OPTS ?=
CONTAINER_CMD ?=
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell docker version >/dev/null 2>&1 && echo docker)
endif
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell podman version >/dev/null 2>&1 && echo podman)
endif

all: manager

# Run tests
test: generate vet manifests
	hack/test.sh

# Build manager binary
manager: generate vet build

build:
	go build -o bin/manager main.go
.PHONY: build

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize set-image
	$(KUSTOMIZE) build config/default | kubectl apply -f -

delete-deploy: manifests kustomize
	$(KUSTOMIZE) build config/default | kubectl delete -f -

set-image: kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
.PHONY: set-image

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt to reformat code
reformat:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the container image
docker-build: image-build
image-build:
	$(CONTAINER_CMD) build $(CONTAINER_BUILD_OPTS) . -t ${IMG}

.PHONY: image-build-buildah
image-build-buildah: build
	cn=$$(buildah from registry.access.redhat.com/ubi8/ubi-minimal:latest) && \
	buildah copy $$cn bin/manager /manager && \
	buildah config --cmd='[]' $$cn && \
	buildah config --entrypoint='["/manager"]' $$cn && \
	buildah commit $$cn ${IMG}

# Push the container image
docker-push: container-push
container-push:
	$(CONTAINER_CMD) push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell command -v controller-gen ;))
	@echo "controller-gen not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/controller-gen ;))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
endif
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell command -v controller-gen ;)
endif

kustomize:
ifeq (, $(shell command -v kustomize ;))
	@echo "kustomize not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/kustomize ;))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
endif
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell command -v kustomize ;)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	$(CONTAINER_CMD) build $(CONTAINER_BUILD_OPTS) -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: check check-revive check-format

check: check-revive check-format

check-format:
	! gofmt $(CHECK_GOFMT_FLAGS) . | sed 's,^,formatting error: ,' | grep 'go$$'

check-revive: revive
	# revive's checks are configured using .revive.toml
	# See: https://github.com/mgechev/revive
	$(REVIVE) -config .revive.toml $$(go list ./... | grep -v /vendor/)

.PHONY: revive
revive:
ifeq (, $(shell command -v revive ;))
	@echo "revive not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/revive ;))
	@{ \
	set -e ;\
	echo "revive not found in GOBIN, getting revive..." ;\
	REVIVE_TMP_DIR=$$(mktemp -d) ;\
	cd $$REVIVE_TMP_DIR ;\
	go mod init tmp ;\
	go get  github.com/mgechev/revive  ;\
	rm -rf $$REVIVE_TMP_DIR ;\
	}
	@echo "revive installed in GOBIN"
else
	@echo "revive found in GOBIN"
endif
REVIVE=$(shell command -v $(GOBIN)/revive ;)
else
	@echo "revive found in PATH"
REVIVE=$(shell command -v revive ;)
endif
