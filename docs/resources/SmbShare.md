# SmbShare Custom Resource

The SmbShare is the fundamental resource used by the samba-operator to
create Samba servers and export storage as SMB shares to the wider world.



```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: example-share1
  namespace: smb-shares
spec:
  shareName: "Example Share 1"
  readOnly: false
  browseable: true
  securityConfig: domain-security1
  commonConfig: common1

  storage:
    pvc:
      name: datapvc
      path: dirname
      spec:
         ... <k8s pvc spec> ...

  scaling:
    availabilityMode: clustered
    minClusterSize: 3
    groupMode: explicit
    group: sg1

status:
  serverGroup: example-share1
```

## Specification

The desired properties of an SmbShare:

* `shareName`: The name of the share in the SMB protocol (in Samba). Optional.
  If unspecified the name of the resource will be used.
* `readOnly`: If set to true clients may only read from the share. Optional.
  Defaults to false.
* `browseable`: If set to true clients may see the share name when listing
  shares on a server. Option. Defaults to true.
* `securityConfig`: The name of an SmbSecurityConfig resource. The
  SmbSecurityConfig resource must exist in the same namespace as the SmbShare.
  Optional. If unspecified the share will default to a simple demo mode
  security.
* `commonConfig`: The name of an SmbCommonConfig resource. The SmbCommonConfig
  resource must exist in the same namespace as the SmbShare. Optional. If
  unspecified the share will default to simple cluster network access.
* `storage`: How the share accesses a supporting storage layer
  * `pvc`: Currently, the samba-operator only supports `PersistentVolumeClaim`s
    as a supporting storage layer
    * `name`: The name of a PersistentVolumeClaim to use for SMB shares.
      Optional. May only be left unset if sibling field `spec` is set.
    * `path`: The name of a directory within the volume corresponding to
      the PersistentVolumeClaim. Optional. If unspecified the root of
      volume will be exposed as a share. Only a single directory name is
      supported, addititonal subdirectories ("/" characters) are not allowed.
    * `spec`: Optional. If specified the pvc spec subsection accepts an
      embedded PersistentVolumeClaim specification. See the [Kubernetes
      Persistent Volumes Documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
      for details. If specified the PVC will automatically be created and
      deleted by the samba-operator and thus has a lifecycle paired to the
      SmbShare.
* `scaling`: Properties related to resources usage and redundancy
  * `availabilityMode`: May be either `standard` or `clustered`. Optional.
    If unspecified defaults to `standard`. Standard availability mode creates
    one Samba server instance to host the share. Clustered availability mode
    create one or more CTDB enabled Samba server to host the share in a HA
    manner. Clustered mode is experimental and needs to be explicitly enabled
    in the operator.
  * `minClusterSize`: The minimum number of Samba server "nodes" to host the
    share. The operator is permitted to run more servers in some conditions.
  * `groupMode`: May be either `never` or `explicit`. Optional. If unspecified
    defaults to `never`. An SmbShare that is ungrouped (never) is always hosted
    by a unique Samba server. A grouped SmbShare may be hosted by Samba server
    instances hosting other SmbShares. Explicit grouping requires a group name
    be supplied in the sibling `group` property.
  * `group`: The name of a share group. Optional, unless `explicit` groupMode
    is in use. The name must be a valid Kubernetes resource name.


### Share grouping restrictions

Traditionally, Samba servers are configured on a per-host basis. That
host may or may not be joined to a domain. All shares defined for
that system have the same system and security properties. The samba-operator
makes it possible to have a variety of different security domains and
other properties by creating multiple pods isolated from each other.
In fact, this is the default behavior of the operator. However, there
are cases where this may lead to higher resource usage and therefore
the `scaling.groupMode` option allows one to combine multiple shares in
one server instance, more like traditional Samba servers.

There are restrictions on what shares can be grouped together. The
operator will perform a compatibility check for shares with `explicit`
`groupMode` and the same `group` name. Most importantly, the Shares
must share the same `securityConfig`, the same `commonConfig`, and
same PVC name.


## Status

* `serverGroup`: Every share is assigned to a virtual server group. This
  group name determines the names of subsequent Kubernetes resources
  created by the samba-operator. The `serverGroup` value can be used
  to determine what pods, deployments, etc. were created in order to
  serve the share.
