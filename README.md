# Samba Operator

An operator for Samba as a service on PVCs in kubernetes.

## Description

This project implements the samba-operator. It it responsible for the
the `SmbService` and `SmbPvc` custom resources:

* [`SmbService`](./config/crd/bases/samba-operator.samba.org_smbservices.yaml)
describes an SMB service deployment that is created
for a given PersistentVolumeClaim (PVC).
* [`SmbPvc`](./config/crd/bases/samba-operator.samba.org_smbpvcs.yaml)
describes a PVC bundled with an SmbService. I.e. you request a pvc along with an
SmbService. When you delete the `SmbPvc`, the created backend PVC will also be deleted.

## Trying it out

### Prerequisite

You need to have a kubernetes cluster running. For example,
[minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/)
is sufficient.

### Start the operator

In order to install the CRDs, other resorces, and start the operator,
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

### Creating an `SmbService`

If you have a PVC `mypvc`, create a `mysmbservice.yml` file as folows (see
		[examples/mysmbservice.yml](examples/mysmbservice.yml)):

```
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbService
metadata:
  name: my-smbservice
spec:
  pvcname: "mypvc"
```

And apply it with `kubectl apply -f mysmbservice.yml`.
You will get a samba container deployment serving out your pvc as share `share`.

### Creating an `SmbPvc`

For an `SmbPvc` example that uses the minikube gluster storage addon, see
[examples/smbpvc.yml](examples/smbpvc1.yml). The yaml file looks like this:

```
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbPvc
metadata:
  name: "mysmbpvc1"
spec:
  pvc:
    accessModes:
      - ReadWriteMany
    resources:
      requests:
        storage: 1Gi
    storageClassName: glusterfile
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


## Containers on quay.io

This operator uses the container built from
[samba-in-kubernetes/samba-container](https://github.com/samba-in-kubernetes/samba-container)
as found on [quay.io](https://quay.io/repository/obnox/samba-centos8).

The container from this codebase is published on
[quay.io](https://quay.io/repository/obnox/samba-operator) too.
