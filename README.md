# Samba Operator

An operator for Samba as a service on PVCs in kubernetes.

## Description

This project implements the samba-operator. It it responsible for the
the `SmbShare`, `SmbSecurityConfig`, and `SmbCommonConfig` custom ressources:

* [`SmbShare`](./config/crd/bases/samba-operator.samba.org_smbshares.yaml)
describes an SMB Share that will be used to share data with clients.
* [`SmbSecurityConfig`](./config/crd/bases/samba-operator.samba.org_smbsecurityconfigs.yaml)
describes domain and/or user based security properties for one or more shares
* [`SmbCommonConfig`](./config/crd/bases/samba-operator.samba.org_smbcommonconfigs.yaml)
describes general configuration properties for smb shares

## Trying it out (Quick Start)

### Prerequisites

You need to have a kubernetes cluster running. For example,
[minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/)
is sufficient.

If you wish to use Active Directory domain based security you need one or more
domain controllers that are visible to Pods within the Kubernetes cluster.

If you wish to access shares from outside the Kubernetes cluster your cluster
must support Services with type `LoadBalancer`.

### Start the operator

In order to install the CRDs, other resources, and start the operator,
invoke:
```
make deploy
```

To use your own image, use:
```
make deploy IMG=<my-registry/and/image:tag>
```

To delete the operator and CRDs from the cluster, run:
```
make delete-deploy
```

Alternatively, if you do not wish to use make tools to deploy the operator, you can also use the kubectl command in the following manner.
```
kubectl apply -k config/default
```

To remove the operator and all related resources, use:
```
kubectl delete -k config/default
```

### Creating new Shares

#### Use a PVC you define

A share can be created that uses pre-existing PVCs, ones that are not directly
managed by the operator.

Assuming you have a PVC named `mypvc`, you can create a new SmbShare using
the example YAML below:

```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: smbshare1
spec:
  storage:
    pvc:
      name: "mypvc"
  readOnly: false
```

### Use a PVC embedded in the SmbShare

A share can be created that embeds a PVC definition. In this case the operator
will automatically manage the PVC along with the share. This example assumes
you have a default storage class enabled.

For example:
```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: smbshare2
spec:
  storage:
    pvc:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  readOnly: false
```

### Testing it with a Local Connection

Assuming a local Linux-based environment you can test out a connection to the
container by forwarding the SMB port and using a local install of `smbclient`
to access the share:

```bash
$ kubectl get pods              NAME                              READY
STATUS    RESTARTS   AGE
my-smbservice-7f779ddc8c-nb6k6    1/1     Running   0          62m
samba-operator-5758b4dbbf-gk9pk   1/1     Running   0          70m
$ kubectl port-forward pod/my-smbservice-7f779ddc8c-nb6k6  4455:445
Forwarding from 127.0.0.1:4455 -> 445
Forwarding from [::1]:4455 -> 445
Handling connection for 4455
```

```
$ smbclient -p 4455 -U sambauser //localhost/share
Enter SAMBA\sambauser's password:
Try "help" to get a list of possible commands.
smb: \> ls
.                                   D        0  Fri Aug 28 14:43:26 2020
..                                  D        0  Fri Aug 28 14:32:53 2020
x                                   A   359386  Fri Aug 28 14:35:18 2020
gefcanilant                         A  5141264  Fri Aug 28 14:43:26 2020

4184064 blocks of size 1024. 4141292 blocks available
smb: \>
```

Above we forward the normal SMB port to an unprivileged local port, assuming
you'll be running this as a normal user.


## Documentation

For additional details on how to set up shares that can authenticate via Active
Directory, or use a load balancer, etc please refer to the
[Samba Operator Documentation](./docs/README.md).



## Containers on quay.io

This operator uses the container built from
[samba-in-kubernetes/samba-container](https://github.com/samba-in-kubernetes/samba-container)
as found on [quay.io](https://quay.io/repository/samba.org/samba-server).

The container from this codebase is published on
[quay.io](https://quay.io/repository/samba.org/samba-operator) too.


## Additional Resources

* [Presentations](./docs/presentations/README.md) about the Samba Operator
* [Developer's Guide](./docs/developers-notes.md) - an incomplete set of tips for working on the operator
