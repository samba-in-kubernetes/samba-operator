#!/usr/bin/env bash


SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEPLOYMENT_YAML="${BASE_DIR}/tests/files/samba-ad-server-deployment.yml"
DEPLOYMENT_NAME="samba-ad-server"
COREDNS_SNIPPET="${BASE_DIR}/tests/files/coredns-snippet.template"
OPENSHIFT_DNS_SNIPPET="${BASE_DIR}/tests/files/openshift-dns-snippet.template"
KUBECTL_CMD=${KUBECTL_CMD:-kubectl}
JQ_CMD=${JQ_CMD:-jq}

_error() {
	echo "$@"
	exit 1
}

_require_command() {
	command -v "${1}" > /dev/null || _error "Can not find ${1}"
}
_require_command "${KUBECTL_CMD}"
_require_command "${JQ_CMD}"


echo "Creating ad server deployment..."
ERROR_MSG=$(${KUBECTL_CMD} create -f "${DEPLOYMENT_YAML}" 2>&1 1>/dev/null)
if [ $? -ne 0 ] ; then
	if [[ "${ERROR_MSG}" =~ "AlreadyExists" ]] ; then
		echo "Deployment exists already. Continuing."
	else
		_error "Error creating ad server deployment."
	fi
fi

${KUBECTL_CMD} get deployment

replicaset="$(${KUBECTL_CMD} describe deployment ${DEPLOYMENT_NAME} | \
	grep -s "NewReplicaSet:" | awk '{ print $2 }')"
[ $? -eq 0 ] || _error "Error getting replicaset"

podname="$(${KUBECTL_CMD} get pod | grep "${replicaset}" | awk '{ print $1 }')"
[ $? -eq 0 ] || _error "Error getting podname"

echo "Samba ad pod is $podname"

echo "waiting for pod to be in Running state"
tries=0
podstatus="none"
until [ $tries -ge 120 ] || echo $podstatus | grep -q 'Running'; do
	sleep 1
	echo -n "."
	tries=$(( tries + 1 ))
	podstatus="$(${KUBECTL_CMD} get pod "${podname}" \
		-o go-template='{{.status.phase}}')"
done
echo
${KUBECTL_CMD} get pod
echo
echo "${podstatus}" | grep -q 'Running' || \
	_error "Pod did not reach Running state"

echo "waiting for samba to become reachable"
tries=0
rc=1
while [ $tries -lt 120 ] && [ $rc -ne 0 ]; do
	sleep 1
	tries=$(( tries + 1 ))
	${KUBECTL_CMD} exec "${podname}" -- \
		smbclient -N -L 127.0.0.1 2>/dev/null 1>/dev/null
	rc=$?
	echo -n "."
done
echo
[ $rc -eq 0 ] || _error "Error: samba ad did not become reachable"


AD_POD_IP=$(${KUBECTL_CMD}  get pod -o json \
	| ${JQ_CMD} -c -M '.items[] | .metadata.name + " " + .status.podIP' \
	| grep samba-ad-server \
	| tr -d "\"" \
	| awk '{print $2}')
[ $? -eq 0 ] || _error "Error getting ad server pod IP"
echo "AD pod IP: ${AD_POD_IP}"

# when running over OpenShift need to patch dns-operator
${KUBECTL_CMD} get deployment dns-operator \
	-n openshift-dns-operator > /dev/null 2>&1
OPENSHIFT_DNS="$?"
if [ ${OPENSHIFT_DNS} -eq 0 ]; then
	PATCHFILE=$(mktemp).json
	sed -e "s/AD_SERVER_IP/${AD_POD_IP}/g" \
		${OPENSHIFT_DNS_SNIPPET} > "${PATCHFILE}"

	${KUBECTL_CMD} patch dns.operator default \
		--type=merge --patch-file "${PATCHFILE}"

	[ $? -eq 0 ] || _error "Failed patching dns-operator with $PATCHFILE"
	unlink "${PATCHFILE}"
	exit 0
fi

TMPFILE=$(mktemp)

cat > "${TMPFILE}" <<EOF
data:
  Corefile: |
EOF

${KUBECTL_CMD} get cm -n kube-system coredns -o jsonpath='{ .data.Corefile }' \
	| sed -e 's/^/    /g' \
	>> "${TMPFILE}"

echo >> "${TMPFILE}"

# don't repeat an existing block for our domain
FIRSTLINE="$(head -1 "${COREDNS_SNIPPET}")"
LASTLINE="    }"

sed -i.backup -e "/$FIRSTLINE/,/$LASTLINE/d" "${TMPFILE}"

sed -e "s/AD_SERVER_IP/${AD_POD_IP}/" "${COREDNS_SNIPPET}" >> "${TMPFILE}"

echo >> "${TMPFILE}"

${KUBECTL_CMD} patch cm -n kube-system coredns -p "$(cat "${TMPFILE}")"
[ $? -eq 0 ] || _error "Error patching coredns config map"
