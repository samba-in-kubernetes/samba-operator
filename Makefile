CONTAINER := quay.io/obnox/samba-operator:v0.0.1

TOOLS_DIR ?= build/_tools

export OPERATOR_SDK_VERSION ?= v0.17.1
export OPERATOR_SDK ?= $(TOOLS_DIR)/operator-sdk-$(OPERATOR_SDK_VERSION)

all: build

operator-sdk: $(OPERATOR_SDK)
.PHONY: operator-sdk

$(OPERATOR_SDK):
	@echo "Ensuring operator-sdk"
	hack/ensure-operator-sdk.sh

generate: generate.crds generate.k8s

generate.k8s: $(OPERATOR_SDK)
	$(OPERATOR_SDK) generate k8s

generate.crds: $(OPERATOR_SDK)
	$(OPERATOR_SDK) generate crds

build: $(OPERATOR_SDK) generate
	$(OPERATOR_SDK) build $(CONTAINER)

push: build
	docker push $(CONTAINER)

install:
	./deploy/install.sh

uninstall:
	./deploy/uninstall.sh

.PHONY: build push generate generate.k8s generate.crds install uninstall
