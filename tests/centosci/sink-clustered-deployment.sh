#!/bin/bash

## Preset variables
#
# CI_IMG_REGISTRY: Internal image registry within CentOS CI
# CI_IMG_OP: Operator image location inside CI_IMG_REGISTRY

set -e

source tests/centosci/sink-common.sh

setup_minikube

deploy_rook

image_pull "${CI_IMG_REGISTRY}" "docker.io" "golang:1.18"

# Build and push operator image to local CI registry
IMG="${CI_IMG_OP}" make image-build
IMG="${CI_IMG_OP}" make container-push

install_kustomize

enable_ctdb

deploy_op

kubectl get pods -A

IMG="${CI_IMG_OP}" make test

# Deploy basic test ad server
./tests/test-deploy-ad-server.sh

# Run integration tests
SMBOP_TEST_CLUSTERED=1 \
    SMBOP_TEST_MIN_NODE_COUNT="${NODE_COUNT}" \
    SMBOP_TEST_EXPECT_MANAGER_IMG="${CI_IMG_OP}" \
    ./tests/test.sh || (./tests/post-test-info.sh; exit 1)

teardown_op

teardown_rook

destroy_minikube

exit 0
