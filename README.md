# samba-operator

An operator for Samba as a service on PVCs in kubernetes.

## Description

This project implements the samba-operator. It it responsible for the
the `SmbService` and `SmbPvc` custom resources
(see [here](deploy/crds/smbservice.samba.org_smbservice_crd.yaml)
and [here](deploy/crds/smbpvc.samba.org_smbpvc_crd.yaml)).
`SmbService` describes and SMB service deployment that is created
for a given PersistentVolumeClaim (PVC) while `SmbPvc` describes a PVC with an
SmbService.

## Trying it out

### Prerequisite

You need to have a kubernetes cluster running. For example,
[minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/)
is sufficient.

### Start the operator

In order to create the operator, perform the following steps:

```
$ kubectl apply -f deploy/crds/smbpvc.samba.org_smbpvcs_crd.yaml
$ kubectl apply -f deploy/crds/smbservice.samba.org_smbservices_crd.yaml
$ kubectl apply -f deploy/service_account.yaml
$ kubectl apply -f deploy/role.yaml
$ kubectl apply -f deploy/role_binding.yaml
$ kubectl apply -f deploy/operator.yaml
```

### Creating your smbservice

If you have a PVC `mypvc`, create a `mysmbservice.yml` file as folows (see
		[examples/mysmbservice.yml](examples/mysmbservice.yml)):

```
apiVersion: smbservice.samba.org/v1alpha1
kind: SmbService
metadata:
  name: my-smbservice
spec:
  pvcname: "mypvc"
```

And apply it with `kubectl apply -f mysmbservice.yml`.
You will get a samba container deployment serving out your pvc as share `share`.

For a `SmbPvc` example that uses the minikube gluster storage addon, see
[examples/smbpvc.yml](examples/smbpvc1.yml). The yaml file looks like this:

```
apiVersion: smbpvc.samba.org/v1alpha1
kind: SmbPvc
metadata:
  name: "mysmbpvc1"
spec:
  pvc:
    accessModes:
      - ReadWriteMany
    resources:
      requests:
        storage: 2Mi
    storageClassName: glusterfile
```

## Containers on quay.io

This operator uses the container built from
[obnoxxx/samba-container](https://github.com/obnoxxx/samba-container)
as found on [quay.io](https://quay.io/repository/obnox/samba-centos8).

The container from this codebase is published on
[quay.io](https://quay.io/repository/obnox/samba-operator) too.
