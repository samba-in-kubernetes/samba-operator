
# Introduction

This document is a proposal for changes and enhancements of the
CRDs (Custom Resource Definitions) of the operator. I'm calling the
current state of the CRDs "Phase 0" and the next set of CRDs "Phase 1".

The current, Phase 0, CRDs support two features. One can define a
SmbService CR that specifies a PVC name and it will create services
to export one share, named "share", based on the PVC name. One can
define an SmbPvc CR that embeds the PVC spec. This will automatically
create a PVC and a matching SmbService CR to export it.


# Phase 1 Custom Resources

The following CRD types are proposed for Phase 1:
* SmbShare - A CR that encapsulates all the information needed to export a
  single share
* SmbSecurityConfig - A CR that encapsulates the knowledge needed to define
  "local" users or become part of an Active Directory domain

The operator will take the SmbShare and SmbSecurityConfig resources as inputs
and create as many smbd, winbind, or other backing services as needed. Users
will have limited input into what backing services the operator will create.
The operator may, or may not, combine one or more share into a single smbd
instance.

One or more SmbSecurityConfig resources can be defined in the cluster.  Each
SmbShare CR can refer to one of those SmbSecurityConfig resources, or rely on
the default settings of the operator.  The SmbSecurityConfig reference will
define the security properties of the smbd instance that hosts the share. If
two SmbShare CRs are defined and each one refers to different SmbSecurityConfig
they must not be combined using one smbd.

The listings below are not meant to be entirely complete but they outline
the general direction to make the operator a fully-fledged tool to
create shares that can be consumed by a variety of clients for various use
cases. Some sections are deliberately marked "TBD" (to be decided) as
placeholders for more detail that will be needed even in Phase 1.

## SmbSecurityConfig

Spec Options:
* `mode` - enumerated string "user" or "active-directory" - Indicates that this
  is an AD joined instance or based on a local mapping of users
* `users` - mapping - A new subsection pertaining to "user" options - required
  when mode = "user"
  * `secret` - string - The name of a secret containing user and group
    definitions that will be used to create the local users and groups for the
    service
  * `key` - string - The key within the secret containing user and group
    definitions that will be used to create the local users and group for the
    service.
* `instanceNamePrefix` - string - Optional string used to construct an
  identifier that will be used to refer to a samba instance. The instance name
  may be used to identify resources in Active Directory, for example.
* `realm` - string - The DNS domain name of an Active Directory domain
* `domains` - list-of-maps - A new subsection pertaining to detailed
  "active-directory" configuration (optional)
  * `backend` - enumerated string - the name of a (supported) ID mapping back-end
  * `name` - string - the name of the domain being configured
  * TBD - options to configure the back end, ID range, etc.
* `joinSources` - list-of-sources - A subsection that describes one or
  more source for AD join information.
  * `userJoin` - A subsection of config data configuring join based on
    username and password information stored in a secret.
    * `Secret` - string - the name of a secret that stores join auth data.
    * `Key` - string - the name of the key within the secret storing the
      data (optional)


## SmbShare

Spec Options:
* `shareName` - string - Optional string giving the share a full SMB compliant
  name. If unset, the name will be derived by the operator.
* `storage` - mapping - A new subsection to configure the storage backing the
  share.
    * `pvc` - mapping - A new subsection used to configure PVC backing storage.
        * `name` - string - The name of a PVC, managed externally to the
          operator, for the share
        * `spec` - mapping, embedded pvc spec - An embedded PVC spec that will
          be used to dynamically create a backing PVC for the share; sharing
          the life-cycle of the PVC with the share
    * TBD - Any other more custom storage back-ends if needed
* `securityConfig` - string - The name of the SmbSecurityConfig CR associated
  with this share
* `scaling` - mapping - Settings pertaining to how resources (servers) managed
  by the operator may be scaled
    * `groupMode` - string - Optional string. May be one of `never` or `basic`.
      Controls if shares may be grouped together under one smbd server. The
      value "none" indicates the operator may not group this share with any
      others. The value "default" indicates that operator may group shares
      based on the `group` parameter.
    * `group` - string - Optional string. When one or more SmbShare CRs contain
      the same group value it is an indication that they can be grouped
      together. If undefined it will be generated by the operator. The grouping
      is used by the operator as a hint to combine shares under one smbd
      service, but only as a hint as the ability to group under one smbd
      depends on other settings
    * `availabilityMode` - Optional string. May be one of `standard` or
      `clustered`. This option tells the operator whether resources backing the
      share should make use of high-availability components. The "standard"
      mode does not enable high-availability and relies only on Kubernetes pod
      migration. The "clustered" availability mode enables smb aware clustering
      mechanisms.
    * `minClusterSize` - int - Minimum number of smbd instances when clustered
      for High-Availbility.
    * TBD - other clustering specific options
* `customConfig` - mapping - A new subsection used to load "non-supported" settings
   * `name` - Name of a ConfigMap. TBD - how to express options in the config map.
* `readOnly` - boolean - When true this share will be read-only (default: false) [1]
* `browseable` - boolean - When false this share will not be browsable (default: true)

## Examples

Two shares sharing the same "local" user mapping:
```yaml
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbSecurityConfig
metadata:
  name: "local-users"
spec:
  mode: "user"
  users:
    secret: "local-users-secret"
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "documents"
spec:
  securityConfig: "local-users"
  storage:
    pvc:
      name: "docs"
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "cad-files"
spec:
  securityConfig: "local-users"
  # We really want a space in our share name. It matches what we had on the
  # previous server.
  shareName: "CAD Files"
  storage:
    pvc:
      name: "cad"
```

The same shares but with an Active Directory configuration:
```yaml
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbSecurityConfig
metadata:
  name: "cool-domain"
spec:
  mode: "active-directory"
  realm: "cool-ad.int.example.com"
  instanceNamePrefix: "kubesamba"
  domains:
    - backend: ad
      name: "cool-ad.int.example.com"
    - backend: ad
      name: "my-trust.int.example.com"
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "documents"
spec:
  securityConfig: "cool-domain"
  storage:
    pvc:
      name: "docs"
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "cad-files"
spec:
  securityConfig: "cool-domain"
  # We really want a space in our share name. It matches what we had on the
  # previous server.
  shareName: "CAD Files"
  storage:
    pvc:
      name: "cad"
```

Two shares, with groups:
```yaml
# SmbSecurityConfig not shown
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "documents"
spec:
  securityConfig: "local-users"
  scaling:
    groupMode: "basic"
    group: "group1"
  storage:
    pvc:
      name: "docs"
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "cad-files"
spec:
  securityConfig: "local-users"
  # We really want a space in our share name. It matches what we had on the
  # previous server.
  shareName: "CAD Files"
  scaling:
    groupMode: "basic"
    group: "group1"
  storage:
    pvc:
      name: "cad"
```

One share with clustering enabled:
```yaml
# SmbSecurityConfig not shown
---
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbShare
metadata:
  name: "documents"
spec:
  securityConfig: "cool-domain"
  storage:
    pvc:
      name: "docs"
  scaling:
    availabilityMode: clustered
    minClusterSize: 5
  browsable: false
```



[1] - This option doesn't currently combine all that well with using an embedded PVC spec as there'd be no way of loading data into the PVC. However, with a named PVC one could pre-load data onto it.
