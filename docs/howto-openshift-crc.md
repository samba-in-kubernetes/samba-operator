# Deploy samba-operator over OpenShift-CRC

The following document describe how to deploy samba-operator and create
SMB shares over OpenShift Container Platform 4 using
[crc](https://crc.dev/crc/). This mode of operation is mainly targeted
at running on developers' Linux desktops and requires
[minimal system resources](https://crc.dev/crc/#minimum-system-requirements-hardware_gsg).
It also requires virtualization enabled on your local machine.

The instructions in this document were tested with the following
settings:

```sh
  $ uname -msr
  Linux 6.1.18-100.fc36.x86_64 x86_64
  $ crc version
  CRC version: 2.15.0+cc05160
  OpenShift version: 4.12.5
  Podman version: 4.3.1
  $ qemu-kvm --version
  QEMU emulator version 6.2.0 (qemu-6.2.0-17.fc36)
```

## Setup OpenShift CRC cluster
Download openshift's crc to your local Linux machine using the
[crc installing instructions](https://crc.dev/crc/#installing_gsg), and
place the `crc` executable within your `PATH`. Ensure that you have a
valid installation by [setting up crc](https://crc.dev/crc/#setting-up_gsg):

```sh
  $ crc version
  $ crc config view
  $ crc setup
```

Make sure that you have an updated pull-secret stored within a local
file (`pull-secret.txt`), and then start a new crc instance with the
following command (may take few minutes):

```sh
  $ crc start -p ./pull-secret.txt
```

Upon successful deployment, you should see information on how to access
your cluster, similar to the following example:

```sh
Started the OpenShift cluster.

The server is accessible via web console at:
  https://console-openshift-console.apps-crc.testing

Log in as administrator:
  Username: kubeadmin
  Password: Y7Dgu-IpHcX-N48UJ-ztphn

Log in as user:
  Username: developer
  Password: developer

Use the 'oc' command line interface:
  $ eval $(crc oc-env)
  $ oc login -u developer https://api.crc.testing:6443

```

Use the `oc` command line utility to ensure cluster's pods are alive
and running:

```sh
  $ eval $(crc oc-env)
  $ export KUBECTL_CMD=oc
  $ $KUBECTL_CMD get pods -A
```

Note that some pods (e.g., `redhat-operators` and `redhat-marketplace`)
may be in `ImagePullBackOff` status, which is fine in the context of
this howto document.

When done with the cluster, you may terminate its resources with:

```sh
  $ crc stop
  ...
  $ crc delete
  ...
```

## Setup OpenShift samba-SCC
Samba operator uses a custom
[security-context-constraints](https://docs.openshift.com/container-platform/4.12/authentication/managing-security-context-constraints.html)
(SCC) for its pods and containers. Before deploying the samba operator,
the user should setup the `samba` SCC on the cluster. In order to
deploy samba SCC manually, execute the following commands:

```sh
  $ cd samba-operator-dir
  $ export KUBECTL_CMD=oc
  $ $KUBECTL_CMD create -f config/openshift/scc.yaml
  securitycontextconstraints.security.openshift.io/samba created
  $KUBECTL_CMD get scc/samba -o yaml
  ...
```

## Deploy samba-operator
Enable developer mode and deploy the samba-operator using the top-level
Makefile target `make-deploy`. Wait for the `samba-operator` pod to be in
`Running` state:

```sh
  $ cd samba-operator-dir
  $ export KUBECTL_CMD=oc
  $ echo DEVELOPER=1 >> devel.mk
  $ make deploy
  ...
  $ $KUBECTL_CMD get pods -n samba-operator-system
  NAME                                                 READY   STATUS    RESTARTS   AGE
  samba-operator-controller-manager-7c877459d4-wln54   2/2     Running   0          27s
```

## Create samba share
Use the `smbtest.yaml` file below to simple SMB share. Wait for the share pod
to be in `Running` state (may take some time):

```sh
  $ export KUBECTL_CMD=oc
  $ $KUBECTL_CMD create -f smbtest.yaml
  ...
  $ $KUBECTL_CMD get pods -n smbtest
  NAME                      READY   STATUS    RESTARTS   AGE
  share1-5f7dbd45bc-bljrv   2/2     Running   0          4m23s
```


```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: smbtest
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: smb-pv
  labels:
    type: local
spec:
  storageClassName: manual
  capacity:
    storage: 8Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: "/mnt/export"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: smb-pvc
  namespace: smbtest
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 4Gi
---
apiVersion: v1
kind: Secret
metadata:
  name: users
  namespace: smbtest
type: Opaque
stringData:
  demousers: |
    {
      "samba-container-config": "v0",
      "users": {
        "all_entries": [
          {
            "name": "user1",
            "password": "123456"
          },
          {
            "name": "user2",
            "password": "123456"
          }
        ]
      }
    }
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbSecurityConfig
metadata:
  name: users
  namespace: smbtest
spec:
  mode: user
  users:
    secret: users
    key: demousers
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbCommonConfig
metadata:
  name: config
  namespace: smbtest
spec:
  network:
    publish: cluster
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: share1
  namespace: smbtest
spec:
  securityConfig: users
  readOnly: false
  storage:
    pvc:
      name: "smb-pvc"
```

## Test samba share using smbtoolbox
Deploy smbtoolbox using the following configuration:

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  namespace: smbtest
  name: smbtoolbox
  annotations:
    openshift.io/scc: samba
spec:
  automountServiceAccountToken: true
  containers:
    - name: smbtoolbox
      image: quay.io/samba.org/samba-toolbox:latest
      command: ["sleep"]
      args: ["100000"]
```

```sh
  $ export KUBECTL_CMD=oc
  $ $KUBECTL_CMD create -f smbtoolbox.yaml
  ...
  $ $KUBECTL_CMD get pods -n smbtest
  NAME                      READY   STATUS    RESTARTS   AGE
  share1-5f7dbd45bc-bljrv   2/2     Running   0          21m
  smbtoolbox                1/1     Running   0          9m25s
```

Use the following shell commands and smbclient to test your smbshare:

```sh
  $ SHARE1_POD="$($KUBECTL_CMD get pods -n smbtest -l samba-operator.samba.org/service=share1 --template '{{(index .items 0).metadata.name}}')"
  $ SHARE1_POD_IP=$($KUBECTL_CMD get pod $SHARE1_POD -n smbtest --template '{{.status.podIP}}')
  $ $KUBECTL_CMD exec -it smbtoolbox -n smbtest -- smbclient -p 445 -U user1%123456 //$SHARE1_POD_IP/share1
  smb: \>
  ...

```
