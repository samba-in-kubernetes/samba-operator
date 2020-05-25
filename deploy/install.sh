#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

set -e

echo
echo "Installing SmbPvc and SmbService CRs"

kubectl apply -f "${BASE_DIR}"/deploy/crds/smbpvc.samba.org_smbpvcs_crd.yaml
kubectl apply -f "${BASE_DIR}"/deploy/crds/smbservice.samba.org_smbservices_crd.yaml

echo "Done."

echo
echo "Installing samba-operator"

kubectl apply -f "${BASE_DIR}"/deploy/service_account.yaml
kubectl apply -f "${BASE_DIR}"/deploy/role.yaml
kubectl apply -f "${BASE_DIR}"/deploy/role_binding.yaml
kubectl apply -f "${BASE_DIR}"/deploy/operator.yaml

echo "Done."
