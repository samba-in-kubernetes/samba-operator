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
```

## Specification

* `network`: Properties related to network configuration.
  * `publish`: May be `cluster` or `external`.
    Determines how Service resources are created for the SmbShare resources.
    Publishing to `cluster` means that the Service is set up for in-cluster
    networking only. Publishing the resource `external` means that the
    Service will be configured as a LoadBalancer service.


NOTE: A LoadBalancer Service requires support from the Kubernetes cluster to
work. In the case of Kubernetes in the Cloud many providers automatically
configured this feature. In the case of a bare-metal Kubernetes cluster you may
need to set up and maintain your own load balancer. The samba-operator has been
tested with [MetalLB](https://metallb.universe.tf/).
