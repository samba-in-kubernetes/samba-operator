# Alllow developer to override some defaults
-include devel.mk

# Current Operator version
VERSION?=0.0.1
# Default bundle image tag
BUNDLE_IMG?=samba-operator-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS:=--channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL:=--default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS?=$(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

COMMIT_ID=$(shell git describe --abbrev=40 --always --exclude='*' --dirty=+ 2>/dev/null)
GIT_VERSION=$(shell git describe --match='v[0-9]*.[0-9]' --match='v[0-9]*.[0-9].[0-9]' 2>/dev/null || echo "(unset)")

CONFIG_KUST_DIR:=config/default
CRD_KUST_DIR:=config/crd
MGR_KUST_DIR:=config/manager
KUSTOMIZE_DEFAULT_BASE:=../default

ifneq ($(DEVELOPER),)
CONFIG_KUST_DIR:=config/developer
MGR_KUST_DIR:=config/developer
endif

GO_CMD:=go
GOFMT_CMD:=gofmt
KUBECTL_CMD?=kubectl
BUILDAH_CMD:=buildah
YAMLLINT_CMD:=yamllint

# Image URL to use all building/pushing image targets
TAG?=latest
IMG?=quay.io/samba.org/samba-operator:$(TAG)

# Produce CRDs that work on Kubernetes 1.16 or later
CRD_OPTIONS?="crd:crdVersions=v1"

CHECK_GOFMT_FLAGS?=-e -s -l

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO_CMD) env GOBIN))
GOBIN=$(shell $(GO_CMD) env GOPATH)/bin
else
GOBIN=$(shell $(GO_CMD) env GOBIN)
endif

# Get current GOARCH
GOARCH?=$(shell $(GO_CMD) env GOARCH)

# Local (alternative) GOBIN for auxiliary build tools
GOBIN_ALT:=$(CURDIR)/.bin


CONTAINER_BUILD_OPTS?=
CONTAINER_CMD?=
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell docker version >/dev/null 2>&1 && echo docker)
endif
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell podman version >/dev/null 2>&1 && echo podman)
endif
# handle the case where podman is present but is (defaulting) to remote and is
# not not functioning correctly. Example: mac platform but not 'podman machine'
# vms are ready
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell podman --version >/dev/null 2>&1 && echo podman)
ifneq ($(CONTAINER_CMD),)
$(warning podman detected but 'podman version' failed. \
	this may mean your podman is set up for remote use, but is not working)
endif
endif

# Helper function to re-format yamls using helper script
define yamls_reformat
	YQ=$(YQ) $(CURDIR)/hack/yq-fixup-yamls.sh $(1)
endef

all: manager build-integration-tests

# Run unit tests
test: generate manifests vet
	$(GO_CMD) test ./... -coverprofile cover.out
.PHONY: test

cover.out: test
coverage.html: cover.out
	$(GO_CMD) tool cover  -html=cover.out  -o coverage.html
.PHONY: coverage.html cover.out

# Build manager binary
manager: generate build vet

build:
	CGO_ENABLED=0 $(GO_CMD) build -o bin/manager -ldflags "-X main.Version=$(GIT_VERSION) -X main.CommitID=$(COMMIT_ID)"  main.go
.PHONY: build

build-integration-tests:
	$(GO_CMD) test -c -o bin/integration-tests -tags integration ./tests/integration
.PHONY: build-integration-tests

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate vet manifests
	$(GO_CMD) run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build $(CRD_KUST_DIR) | $(KUBECTL_CMD) apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build $(CRD_KUST_DIR) | $(KUBECTL_CMD) delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize set-image
	$(KUSTOMIZE) build $(CONFIG_KUST_DIR) | $(KUBECTL_CMD) apply -f -

delete-deploy: manifests kustomize
	$(KUSTOMIZE) build $(CONFIG_KUST_DIR) | $(KUBECTL_CMD) delete -f -

# the bar symbol below is an order only prerequisite
#  https://www.gnu.org/software/make/manual/make.html#index-order_002donly-prerequisites
# this is needed because kustomize is phony but we really do not want to
# have that force the kustomization.yaml to be considered "dirty"
%/kustomization.yaml: | kustomize
	mkdir -p $*
	touch $@
	cd $* && $(KUSTOMIZE) edit add base $(KUSTOMIZE_DEFAULT_BASE)

# We could make developer-dir always create a developer dir, but I want to be
# consistent and "train" the caller to use DEVELOPER=1 whenever "developer
# mode" is being invoked even when it is not strictly needed for
# implementation.
ifneq ($(DEVELOPER),)
developer-dir: $(MGR_KUST_DIR)/kustomization.yaml
else
developer-dir:
	@echo "When creating a developer-dir, DEVELOPER=1 is required." && exit 1
endif
.PHONY: developer-dir

set-image: kustomize $(MGR_KUST_DIR)/kustomization.yaml
	cd $(MGR_KUST_DIR) && $(KUSTOMIZE) edit set image quay.io/samba.org/samba-operator=$(IMG)
.PHONY: set-image

# Generate manifests e.g. CRD, RBAC etc.
manifests: yq controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook \
		paths="./..." output:crd:artifacts:config=$(CRD_KUST_DIR)/bases
	$(call yamls_reformat, $(CURDIR)/config)

# Run go fmt to reformat code
reformat:
	$(GO_CMD) fmt ./...

# Run go vet against code
vet:
	$(GO_CMD) vet ./...

# Format yaml files for yamllint standard
.PHONY: yaml-fmt
yaml-fmt: yq
	$(call yamls_reformat, $(CURDIR))

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the container image
docker-build: image-build
image-build:
	$(CONTAINER_CMD) build \
		--build-arg=GIT_VERSION="$(GIT_VERSION)" \
		--build-arg=COMMIT_ID="$(COMMIT_ID)" \
		--build-arg=ARCH="$(GOARCH)" \
		$(CONTAINER_BUILD_OPTS) . -t $(IMG)

.PHONY: image-build-buildah
image-build-buildah: build
	cn=$$($(BUILDAH_CMD) from registry.access.redhat.com/ubi8/ubi-minimal:latest) && \
	$(BUILDAH_CMD) copy $$cn bin/manager /manager && \
	$(BUILDAH_CMD) config --cmd='[]' $$cn && \
	$(BUILDAH_CMD) config --entrypoint='["/manager"]' $$cn && \
	$(BUILDAH_CMD) commit $$cn $(IMG)


.PHONY: image-build-multiarch image-push-multiarch
image-build-multiarch: image-build-multiarch-manifest \
			image-build-arch-amd64 image-build-arch-arm64
	$(BUILDAH_CMD) manifest inspect $(IMG)

image-build-multiarch-manifest:
	$(BUILDAH_CMD) manifest create $(IMG)

image-build-arch-%:  qemu-utils
	$(BUILDAH_CMD) bud \
		--manifest $(IMG) \
		--arch "$*" \
		--tag "$(IMG)-$*" \
		--build-arg=GIT_VERSION="$(GIT_VERSION)" \
		--build-arg=COMMIT_ID="$(COMMIT_ID)" \
		--build-arg=ARCH="$*" .

image-push-multiarch:
	$(BUILDAH_CMD) manifest push --all $(IMG) "docker://$(IMG)"


# Push the container image
docker-push: container-push
container-push:
	$(CONTAINER_CMD) push $(IMG)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	@echo "This rule is currently disabled. It is retained for reference only."
	@false
	# See vcs history for how this could be restored or adapted in the future.

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	@echo "This rule is currently disabled. It is retained for reference only."
	@false
	# See vcs history for how this could be restored or adapted in the future.

.PHONY: check check-revive check-golangci-lint check-format check-yaml check-gosec check-dockerfile-go-version

check: check-revive check-golangci-lint check-format vet check-yaml check-gosec check-dockerfile-go-version

check-format:
	! $(GOFMT_CMD) $(CHECK_GOFMT_FLAGS) . | sed 's,^,formatting error: ,' | grep 'go$$'

check-revive: revive
	# revive's checks are configured using .revive.toml
	# See: https://github.com/mgechev/revive
	$(REVIVE) -config .revive.toml $$($(GO_CMD) list ./... | grep -v /vendor/)

check-golangci-lint: golangci-lint
	$(GOLANGCI_LINT) -c .golangci.yaml run ./...

check-yaml:
	$(YAMLLINT_CMD) -c ./.yamllint.yaml ./

check-gosec: gosec
	$(GOSEC) -quiet -exclude=G101 -fmt json ./...

check-dockerfile-go-version:
	# use go-version-check.sh --show to list vaild golang builder images
	$(CURDIR)/hack/go-version-check.sh --check

check-gitlint: gitlint
	$(GITLINT) -C .gitlint --commits origin/master.. lint

# find or download auxiliary build tools
.PHONY: build-tools controller-gen kustomize revive golangci-lint yq
build-tools: controller-gen kustomize revive golangci-lint yq

define installtool
	@GOBIN=$(GOBIN_ALT) GO_CMD=$(GO_CMD) $(CURDIR)/hack/install-tools.sh $(1)
endef

controller-gen:
ifeq (, $(shell command -v controller-gen ;))
	@echo "controller-gen not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/controller-gen ;))
	@$(call installtool, --controller-gen)
	@echo "controller-gen installed in $(GOBIN_ALT)"
endif
CONTROLLER_GEN=$(GOBIN_ALT)/controller-gen
else
CONTROLLER_GEN=$(shell command -v controller-gen ;)
endif

kustomize:
ifeq (, $(shell command -v kustomize ;))
	@echo "kustomize not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/kustomize ;))
	@$(call installtool, --kustomize)
	@echo "kustomize installed in $(GOBIN_ALT)"
endif
KUSTOMIZE=$(GOBIN_ALT)/kustomize
else
KUSTOMIZE=$(shell command -v kustomize ;)
endif

revive:
ifeq (, $(shell command -v revive ;))
	@echo "revive not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/revive ;))
	@$(call installtool, --revive)
	@echo "revive installed in $(GOBIN_ALT)"
endif
REVIVE=$(GOBIN_ALT)/revive
else
	@echo "revive found in PATH"
REVIVE=$(shell command -v revive ;)
endif

golangci-lint:
ifeq (, $(shell command -v golangci-lint ;))
	@echo "golangci-lint not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/golangci-lint ;))
	@$(call installtool, --golangci-lint)
	@echo "golangci-lint installed in $(GOBIN_ALT)"
endif
GOLANGCI_LINT=$(GOBIN_ALT)/golangci-lint
else
GOLANGCI_LINT=$(shell command -v golangci-lint ;)
endif

yq:
ifeq (, $(shell command -v yq ;))
	@echo "yq not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/yq ;))
	@$(call installtool, --yq)
	@echo "yq installed in $(GOBIN_ALT)"
endif
YQ=$(GOBIN_ALT)/yq
else
YQ=$(shell command -v yq ;)
endif

gosec:
ifeq (, $(shell command -v gosec ;))
	@echo "gosec not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/gosec ;))
	@$(call installtool, --gosec)
	@echo "gosec installed in $(GOBIN_ALT)"
endif
GOSEC=$(GOBIN_ALT)/gosec
else
GOSEC=$(shell command -v gosec ;)
endif

gitlint:
ifeq (, $(shell command -v gitlint ;))
	@echo "gitlint not found in PATH, checking $(GOBIN_ALT)"
ifeq (, $(shell command -v $(GOBIN_ALT)/gitlint ;))
	@$(call installtool, --gitlint)
	@echo "gitlint installed in $(GOBIN_ALT)"
endif
GITLINT=$(GOBIN_ALT)/gitlint
else
GITLINT=$(shell command -v gitlint ;)
endif

.PHONY: qemu-utils
qemu-utils:
ifeq (, $(shell command -v qemu-x86_64-static ;))
	$(error "qemu-x86_64-static not found in PATH")
endif
ifeq (, $(shell command -v qemu-aarch64-static ;))
	$(error "qemu-aarch64-static not found in PATH")
endif
