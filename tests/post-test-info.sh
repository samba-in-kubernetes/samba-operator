#!/bin/sh
#
# A very simplistic script to help dump logs / diagnose test failures.
#


dump_operator_logs() {
    ns="samba-operator-system"
    echo "----> operator logs:"
    kubectl -n "$ns" logs \
        deploy/samba-operator-controller-manager \
        --all-containers=true \
        --prefix=true
}

dump_smb_pod_logs() {
    ns="samba-operator-system"
    echo "----> samba pod logs/info ($ns):"
    podnames="$(kubectl -n "$ns" get pods \
         -l app.kubernetes.io/name=samba \
        -o go-template='{{range .items}}{{.metadata.name}}{{printf "\n"}}{{end}}')"
    for pod in ${podnames}; do
        echo "------> ${pod}"
        kubectl -n "$ns" describe pod "${pod}"
        kubectl -n "$ns" logs \
            "pod/${pod}" \
            --all-containers=true \
            --prefix=true
    done
}


echo "----- dumping logs --------------"
dump_operator_logs
dump_smb_pod_logs
echo "---------------------------------"
