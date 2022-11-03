# SmbSecurityConfig Custom Resource

The SmbSecurityConfig resource is used to define properties related to
how the share and the Samba servers hosting the shares will restrict
access to the shares. This includes properties that support joining
server instances to Active Directory.


```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbSecurityConfig
metadata:
  name: domain-security1
  namespace: smb-shares
spec:
  mode: active-directory
  realm: DOMAIN.EXAMPLE.ORG
  joinSources:
    - secret: join1
      key: exampleorg
  dns:
    register: external-ip

  users:
    secret: users1
    key: demousers
```

## Specification

* `mode`: May be either `user` or `active-directory`. Determines if
  the access control for a Samba server instance will be based on locally
  defined users and groups or an Active Directory domain.
* `realm`: Relevant to active-directory mode only. Specifies the domain
  (aka realm) to join. Case insensitive.
* `joinSources`: A list of sources for Active Directory authentication
  values that will allow a new server instance to join a domain.
  Each source will be tried in order until join succeeds or the list
  is exhausted.
  * `secret`: The name of a Kubernetes Secret resource in the same
    namespace as the SmbSecurityConfig.
  * `key`: The name of a key within the Kubernetes Secret holding
    the values that will be used to join Active Directory.
* `dns`: Properties related to the DNS subsystem in Active Directory. Optional.
  * `register`: May be `never`, `external-ip`, or `cluster-ip`.
    Determines if/what IP address to register the Samba server(s) with the
    Active Directory DNS. A `never` value will not register the server.
    The `external-ip` value will attempt to register the external IP
    address of the instance via the Kubernetes Service. The `cluster-ip` will
    attempt to register the internal cluster IP of the instance via the
    Kubernetes Service.
* `users`: Locally defined users and groups. Only used in `user` mode.
  * `secret`: The name of a Kubernetes Secret resource in the same
    namespace as the SmbSecurityConfig.
  * `key`: The name of a key within the Kubernetes Secret holding a JSON
    blob describing the users and groups to define.

Both `user` mode and `active-directory` mode require the use of Kubernetes
secrets. For `user` mode the secret must contain a description of what users
and groups need to be defined. In `active-directory` mode the secrets contain
values required to join to Active Directory. The secrets are only ever read
by the Pods the operator creates and never by the operator itself.


## Join Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: join1
type: Opaque
stringData:
  join.json: |
    {"username": "Administrator", "password": "P4ssw0rd"}
```

When defining an SmbSecurityConfig for `active-directory` mode at least
one Secret must be created containing authentication tokens that permit
a Samba server to join the domain. A key within the secret must
be a JSON blob containing the following fields:

* `username`: A user in Active Directory with permissions to join a system
  to the domain.
* `password`: The user's password.


## Users and Groups Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: users1
type: Opaque
stringData:
  demousers: |
    {
      "samba-container-config": "v0",
      "users": {
        "all_entries": [
          {
            "name": "sambauser",
            "password": "1nsecurely"
          },
          {
            "name": "alice",
            "password": "wond3r1and"
          }
        ]
      }
    }
```

When defining users and groups for a server instance a secret must
be created containing at least one key. The value of the key must
be a JSON blob containing the following fields:

* `samba-container-config`: Value must be `v0`.
* `users`: Top level user definitions.
  * `all_entries`: A list of sub objects.
    * `name`: A user name.
    * `password`: A password.

<!-- TODO: Describe groups. Describe pre-hashed passwords? -->
