CONTAINER := quay.io/obnox/samba-operator:v0.0.1

OUTPUT ?= build/_output
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
.PHONY: generate

generate.k8s: $(OPERATOR_SDK)
	$(OPERATOR_SDK) generate k8s
.PHONY: generate.k8s

generate.crds: $(OPERATOR_SDK)
	$(OPERATOR_SDK) generate crds
.PHONY: generate.crds

build: $(OPERATOR_SDK) generate
	$(OPERATOR_SDK) build $(CONTAINER)
.PHONY: build

clean:
	rm -rf $(OUTPUT)
	rm -f go.sum
.PHONY: clean

realclean: clean
	rm -rf $(TOOLS_DIR)
.PHONY: realclean

push: build
	docker push $(CONTAINER)
.PHONY: push

install:
	./deploy/install.sh
.PHONY: install

uninstall:
	./deploy/uninstall.sh
.PHONY: uninstall
