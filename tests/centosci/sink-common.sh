#!/bin/bash

## Preset variables
#
# REGISTRY_AUTH_FILE: Credentials in json for authenticating to CI_IMG_REGISTRY

ROOK_VERSION=${ROOK_VERSION:-"latest"}
ROOK_DEPLOY_TIMEOUT=${ROOK_DEPLOY_TIMEOUT:-1200}
ROOK_TEMP_DIR=${ROOK_TEMP_DIR:-""}

KUBE_VERSION=${KUBE_VERSION:-"latest"}
KUBECTL_RETRY=5
KUBECTL_RETRY_DELAY=10
CONTAINER_CMD=${CONTAINER_CMD:-"podman"}

MINIKUBE_ARCH=${MINIKUBE_ARCH:-"amd64"}
MINIKUBE_VERSION=${MINIKUBE_VERSION:-"latest"}

VM_DRIVER=${VM_DRIVER:-"kvm2"}
NODE_COUNT=${NODE_COUNT:-"3"}
MEMORY=${MEMORY:-"4096"}
CPUS=${CPUS:-"2"}
NUM_DISKS=${NUM_DISKS:-"2"}
DISK_SIZE=${DISK_SIZE:-"10g"}
DISK_CONFIG=${DISK_CONFIG:-" --extra-disks=${NUM_DISKS} --disk-size=${DISK_SIZE}"}

image_pull() {
	${CONTAINER_CMD} pull "${1}"/"${3}" && \
		${CONTAINER_CMD} tag "${1}"/"${3}" "${2}"/"${3}"
}

install_binaries() {
	curl -Lo minikube https://storage.googleapis.com/minikube/releases/"${MINIKUBE_VERSION}"/minikube-linux-"${MINIKUBE_ARCH}"
	install minikube /usr/local/sbin/minikube

	# Download and install kubectl
	curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/"${KUBE_VERSION}"/bin/linux/"${MINIKUBE_ARCH}"/kubectl
	install kubectl /usr/local/sbin/kubectl
}

kubectl_retry() {
	retries=0
	ret=0

	stdout=$(mktemp rook-kubectl-stdout.XXXXXXXX)
	stderr=$(mktemp rook-kubectl-stderr.XXXXXXXX)

	while ! kubectl "${@}" 2>"${stderr}" 1>"${stdout}"
	do
		retries=$((retries+1))
		if [ ${retries} -eq ${KUBECTL_RETRY} ]; then
			ret=1
			break
		fi

		cat "${stderr}" > /dev/stderr
		true > "${stderr}"
		echo "kubectl_retry ${*} failed, will retry in ${KUBECTL_RETRY_DELAY} seconds"
		cat /dev/null > /dev/stdout

		sleep ${KUBECTL_RETRY_DELAY}
	done

	cat "${stdout}" > /dev/stdout
	cat "${stderr}" > /dev/stderr

	rm -f "${stdout}" "${stderr}"

	return ${ret}
}

minikube_load() {
	for n in ${1}; do
		${CONTAINER_CMD} image save ${2} | ssh \
			-o UserKnownHostsFile=/dev/null \
			-o StrictHostKeyChecking=no \
			-i "$(minikube ssh-key -n "$n")" \
			-l docker "$(minikube ip -n "$n")" "docker image load"
	done
}

setup_minikube() {
	install_binaries
	image_pull "${CI_IMG_REGISTRY}" "docker.io" "kindest/kindnetd:v20210326-1e038dc5"

	# Start a kuberentes cluster using minikube
	minikube start --force --driver="${VM_DRIVER}" --nodes="${NODE_COUNT}" \
		--memory="${MEMORY}" --cpus="${CPUS}" ${DISK_CONFIG} \
		--delete-on-failure --install-addons=false -b kubeadm \
		--kubernetes-version="${KUBE_VERSION}" ${EXTRA_CONFIG}

	nodes=$(kubectl get nodes \
			-o jsonpath='{range.items[*].metadata}{.name} {end}')

	minikube_load "${nodes}" "docker.io/kindest/kindnetd:v20210326-1e038dc5"

	echo "Wait for k8s cluster..."
	for ((retry = 0; retry <= 20; retry = retry + 2)); do
		kubectl_retry -n kube-system rollout status deployment coredns
		deployment_status=$?
		if [ "${deployment_status}" -eq 0 ]; then
			echo -e "\nThree node k8s cluster ready [${retry}s]"
			break
		fi

		sleep 2
		echo -n "."
	done

	if [ "${retry}" -gt 20 ]; then
		echo -e "\nFailed to setup multinode k8s cluster (timeout: 20s)"
		exit 1
	fi

	kubectl cluster-info

	# Configure nodes to authenticate to CI registry(copy config.json)
	for n in $nodes; do
		cat < "${REGISTRY_AUTH_FILE}" | ssh \
			-o UserKnownHostsFile=/dev/null \
			-o StrictHostKeyChecking=no \
			-i "$(minikube ssh-key -n "$n")" \
			-l docker "$(minikube ip -n "$n")" \
			"sudo tee /var/lib/kubelet/config.json > /dev/null";
	done
}

destroy_minikube() {
	minikube delete
}

deploy_rook() {
	ROOK_URL="https://raw.githubusercontent.com/rook/rook/${ROOK_VERSION}/deploy/examples"

	if [ -z "${ROOK_TEMP_DIR}" ]; then
		ROOK_TEMP_DIR=$(mktemp -d)
	fi

	curl -o "${ROOK_TEMP_DIR}/crds.yaml" "${ROOK_URL}/crds.yaml"
	curl -o "${ROOK_TEMP_DIR}/common.yaml" "${ROOK_URL}/common.yaml"
	curl -o "${ROOK_TEMP_DIR}/operator.yaml" "${ROOK_URL}/operator.yaml"

	kubectl create -f "${ROOK_TEMP_DIR}/common.yaml"
	kubectl create -f "${ROOK_TEMP_DIR}/crds.yaml"
	kubectl create -f "${ROOK_TEMP_DIR}/operator.yaml"

	curl -o "${ROOK_TEMP_DIR}/cluster.yaml" "${ROOK_URL}/cluster.yaml"

	# Use /data/rook as host path in case of minikube cluster
	sed -i '/^ *dataDirHostPath/s/\/var\/lib\/rook/\/data\/rook/' \
		"${ROOK_TEMP_DIR}"/cluster.yaml

	# Consume only extra added disks
	sed -i '/^ *useAllDevices/s/true/false/' "${ROOK_TEMP_DIR}"/cluster.yaml

	dev_letter=({b..z})
	dev_lst="\    devices:"
	for ((disks = 0; disks < NUM_DISKS; disks = disks + 1)); do
		dev_lst="${dev_lst}\n      - name: \"vd${dev_letter[disks]}\""
	done

	sed -i "/^ *useAllDevices/a ${dev_lst}" "${ROOK_TEMP_DIR}"/cluster.yaml

	kubectl create -f "${ROOK_TEMP_DIR}/cluster.yaml"

	echo "Wait for rook deploy..."
	# Wait for Ceph cluster to be HEALTHY
	for ((retry = 0; retry <= ROOK_DEPLOY_TIMEOUT; retry = retry + 10)); do
		CEPH_STATE=$(kubectl_retry -n rook-ceph get cephclusters \
				-o jsonpath='{.items[0].status.state}')
		CEPH_HEALTH=$(kubectl_retry -n rook-ceph get cephclusters \
				-o jsonpath='{.items[0].status.ceph.health}')
		if [ "$CEPH_STATE" = "Created" ]; then
			if [ "$CEPH_HEALTH" = "HEALTH_OK" ]; then
				echo -e "\nCeph cluster created [${retry}s]"
				break
			fi
		fi

		sleep 10
		echo -n "."
	done

	if [ "${retry}" -gt "$ROOK_DEPLOY_TIMEOUT" ]; then
		echo -e "\nDeploying rook failed (timeout: ${ROOK_DEPLOY_TIMEOUT}s)"
		exit 1
	fi

	# Install required Ceph tools
	curl -o "${ROOK_TEMP_DIR}/toolbox.yaml" "${ROOK_URL}/toolbox.yaml"
	curl -o "${ROOK_TEMP_DIR}/pool.yaml" "${ROOK_URL}/pool.yaml"
	curl -o "${ROOK_TEMP_DIR}/filesystem.yaml" "${ROOK_URL}/filesystem.yaml"

	kubectl create -f "${ROOK_TEMP_DIR}/toolbox.yaml"
	kubectl create -f "${ROOK_TEMP_DIR}/filesystem.yaml"
	kubectl create -f "${ROOK_TEMP_DIR}/pool.yaml"

	# Install and make Ceph filesystem storage class the default
	curl -o "${ROOK_TEMP_DIR}/storageclass.yaml" \
		"${ROOK_URL}/csi/cephfs/storageclass.yaml"

	kubectl create -f "${ROOK_TEMP_DIR}/storageclass.yaml"
	kubectl patch storageclass rook-cephfs \
		-p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

teardown_rook() {
	# Probable issue in cleaning up the ceph cluster is that the rook-ceph
	# namespace or the ceph cluster CRD may remain indefinitely in a
	# terminating state. It is therefore recommended to directly reset the
	# entire k8s cluster without worrying about the following individual
	# resource cleanups.
	#
	#items=("storageclass.yaml" "pool.yaml" "filesystem.yaml" "toolbox.yaml" \
	#	"cluster.yaml" "operator.yaml" "crds.yaml" "common.yaml")
	#for i in "${items[@]}"; do
	#	kubectl delete -f "${ROOK_TEMP_DIR}/$i"
	#done

	rm -rf "${ROOK_TEMP_DIR}"
}

install_kustomize() {
	make kustomize
	KUSTOMIZE=$(readlink -f .bin/kustomize)
	export KUSTOMIZE
}

enable_ctdb() {
	pushd config/default || exit 1
	${KUSTOMIZE} edit add configmap controller-cfg --behavior=merge \
		--from-literal="SAMBA_OP_CLUSTER_SUPPORT=ctdb-is-experimental"
	sed -i '$a\  namespace: system' kustomization.yaml
	popd || exit 1
}

deploy_op() {
	IMG="${CI_IMG_OP}" make deploy

	echo "Wait for operator deployment..."
	for ((retry = 0; retry <= 60; retry = retry + 2)); do
		podstatus=$(kubectl_retry -n samba-operator-system get pod \
				-l control-plane=controller-manager \
				-o jsonpath='{.items[0].status.phase}')
		kubectl_retry -n samba-operator-system rollout status \
			deployment samba-operator-controller-manager
		deployment_status=$?
		if [ "${podstatus}" = "Running" ]; then
			if [ "${deployment_status}" -eq 0 ]; then
				echo -e "\nOperator deployed and" \
					"running successfully [${retry}s]"
				break
			fi
		fi

		sleep 2
		echo -n "."
	done

	if [ "${retry}" -gt 60 ]; then
		echo -e "\nOperator deployment failed (timeout: 60s)"
		exit 1
	fi
}

teardown_op() {
	make delete-deploy
}

# kubelet.resolv-conf needs to point to a file, not a symlink
# default minikube VM has /etc/resolv.conf -> /run/systemd/resolve/resolv.conf
RESOLV_CONF="/run/systemd/resolve/resolv.conf"
if [[ ! -e "${RESOLV_CONF}" ]]; then
	# in case /run/systemd/resolve/resolv.conf does not exist, use the
	# standard /etc/resolv.conf (with symlink resolved)
	RESOLV_CONF="$(readlink -f /etc/resolv.conf)"
fi

EXTRA_CONFIG="${EXTRA_CONFIG} --extra-config=kubelet.resolv-conf=${RESOLV_CONF}"

dnf -y install epel-release

# Install basic tools
dnf -y install make jq

# Install libvirt, QEMU-KVM and related packages
dnf -y install qemu-kvm qemu-img libvirt libvirt-devel socat conntrack

# Install go build environment
dnf -y install go

if [[ "${KUBE_VERSION}" == "latest" ]]; then
	# update the version string from latest with the real version
	KUBE_VERSION=$(curl -L https://storage.googleapis.com/kubernetes-release/release/stable.txt 2> /dev/null)
else
	KUBE_VERSION=$(curl -L https://api.github.com/repos/kubernetes/kubernetes/releases | \
			jq -r '.[].tag_name' | grep "${KUBE_VERSION}" | \
			sort -V | tail -1)
fi

if [[ "${ROOK_VERSION}" == "latest" ]]; then
	ROOK_VERSION=$(curl -L https://api.github.com/repos/rook/rook/releases | \
			jq -r '.[].tag_name' | sort -V | tail -1)
else
	ROOK_VERSION=$(curl -L https://api.github.com/repos/rook/rook/releases | \
			jq -r '.[].tag_name' | grep "${ROOK_VERSION}" | \
			sort -V | tail -1)
fi

# Start libvrit daemon
systemctl enable --now libvirtd

# minikube wants the user to be in the libvirt group
getent group libvirt || groupadd --system libvirt
usermod -aG libvirt root
