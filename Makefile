CONTAINER := quay.io/obnox/samba-operator:v0.0.1

generate: generate.crds generate.k8s

generate.k8s:
	operator-sdk generate k8s

generate.crds:
	operator-sdk generate crds

build: generate
	operator-sdk build $(CONTAINER)

push: build
	docker push $(CONTAINER)

.PHONY: build push
