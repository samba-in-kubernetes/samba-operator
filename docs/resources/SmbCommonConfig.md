# SmbCommonConfig Custom Resource

The SmbCommonConfig resource is used to define properties related to networking
that are shared across one or more SmbShares.

```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbCommonConfig
metadata:
  name: domain-security1
  namespace: smb-shares
spec:
  network:
    publish: external
  podSettings:
    nodeSelector:
      "kubernetes.io/os": "linux"
      "kubernetes.io/arch": "amd64"
    affinity:
      ... <k8s affinity spec> ...
```

## Specification

* `network`: Properties related to network configuration.
  * `publish`: May be `cluster` or `external`.
    Determines how Service resources are created for the SmbShare resources.
    Publishing to `cluster` means that the Service is set up for in-cluster
    networking only. Publishing the resource `external` means that the
    Service will be configured as a LoadBalancer service.
* `podSettings`: Optional settings controlling how pods created by the operator
   are constructed.
  * `nodeSelector`: Optional map of Kubernetes labels to values.
    Equivalent to the pod spec value of the same name.
    See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
  * `affinity`: Optional specification controlling node affinity and pod affinities.
    Equivalent to the pod spec value of the same name.
    See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity


NOTE: A LoadBalancer Service requires support from the Kubernetes cluster to
work. In the case of Kubernetes in the Cloud many providers automatically
configured this feature. In the case of a bare-metal Kubernetes cluster you may
need to set up and maintain your own load balancer. The samba-operator has been
tested with [MetalLB](https://metallb.universe.tf/).


Full example of an SmbCommonConfig with both node selector and node affinity
settings applied:
```yaml
apiVersion: samba-operator.samba.org/v1alpha1
kind: SmbCommonConfig
metadata:
  name: domain-security1
  namespace: smb-shares
spec:
  network:
    publish: external
  podSettings:
    nodeSelector:
      "kubernetes.io/os": "linux"
      "kubernetes.io/arch": "amd64"
    affinity:
      nodeAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 10
            preference:
              matchExpressions:
                - key: storage-server
                  operator: In
                  values:
                    - samba
                    - smb
                    - default
```
