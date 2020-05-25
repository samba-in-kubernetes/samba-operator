#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

set -e

echo
echo "Uninstalling samba-operator"

kubectl delete -f "${BASE_DIR}"/deploy/operator.yaml
kubectl delete -f "${BASE_DIR}"/deploy/role_binding.yaml
kubectl delete -f "${BASE_DIR}"/deploy/role.yaml
kubectl delete -f "${BASE_DIR}"/deploy/service_account.yaml

echo "Done."

echo
echo "Uninstalling SmbPvc and SmbService CRs"

kubectl delete -f "${BASE_DIR}"/deploy/crds/smbpvc.samba.org_smbpvcs_crd.yaml
kubectl delete -f "${BASE_DIR}"/deploy/crds/smbservice.samba.org_smbservices_crd.yaml

echo "Done."
