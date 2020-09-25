
# Rebooting samba-operator with operator-sdk v1.0


Using documentation at:
* https://v1-0-x.sdk.operatorframework.io/docs/building-operators/golang/
* https://v1-0-x.sdk.operatorframework.io/docs/building-operators/golang/project_migration_guide/

The existing CRDs/CRs were apparently multi-group already, but since the
project is immature and there's probably no cost is reverting this change, I'll
be making them single group. (Refer to
https://v1-0-x.sdk.operatorframework.io/docs/building-operators/golang/tutorial/#multi-group-apis
and
https://v1-0-x.sdk.operatorframework.io/docs/building-operators/golang/tutorial/#multi-group-apis
).


## Steps performed

* Created new git branch early in history (prior to sdk v0.17 init'ed)
* `operator-sdk-v1.0.0 init --domain samba.org --repo github.com/obnoxxx/samba-operator`
* `operator-sdk-v1.0.0 create api --group=samba-operator --version=v1alpha1 --kind=SmbService --resource --controller`
* `git show master:pkg/apis/smbservice/v1alpha1/smbservice_types.go > orig_smbservice_types.go`
* Copy contents of spec and status types to new `smbservice_types.go` file.
* For comparison: `diff -u orig_smbservice_types.go api/v1alpha1/smbservice_types.go`
* `git show master:pkg/controller/smbservice/smbservice_controller.go > orig_smbservice_controller.go`
* Copy significant sections of `orig_smbservice_controller.go` to new `controllers/smbservice_controller.go` file (trickier):
  * Custom fields from ReconcileXYZ to XYZReconciler
  * Body of reconcile logic
  * SetupWithManager function (just `Owns(&appsv1.Deployment{})` for now?)
  * Add RBAC markers
* Fix up dockerfile to use ubi8
* Cleanup temporary files: `rm -rf orig_*.go`
* Fix up Makefile (some of us don't have a  `docker` command!)
* `make manifests`
* `make`
* `operator-sdk-v1.0.0 create api --group=samba-operator --version=v1alpha1 --kind=SmbPvc --resource --controller`
* `git show master:pkg/apis/smbpvc/v1alpha1/smbpvc_types.go > orig_smbpvc_types.go.txt`
* Copy contents of spec and status types to new `smbservice_types.go` file.
* `git show master:pkg/controller/smbpvc/smbpvc_controller.go > orig_smbpvc_controller.go.txt`
* Copy significant sections of `orig_smbservice_controller.go.txt` to new `controllers/smbservice_controller.go` file
* `make manifests`
* `make`


----------------------------------------------------------------------

## Followed docs

https://opensource.com/article/20/3/kubernetes-operator-sdk

https://sdk.operatorframework.io/docs/golang/quickstart/


## Steps

### Initialization

* `operator-sdk new samba-operator --type go --repo github.com/obnoxxx/samba-operator`
* `cd samba-operator`
* `operator-sdk add api --kind SmbService --api-version smbservice.samba.org/v1alpha1`
* edit `pkg/apis/presentation/v1alpha1/smbservice_types.go`, adding pvcname
* `operator-sdk generate crds`
* `operator-sdk generate k8s`
* `operator-sdk add controller --kind SmbService --api-version smbservice.samba.org/v1alpha1`
* `kubectl apply -f deploy/crds/smbservice.samba.org_smbservice_crd.yaml`

* there was a weirdness in the initialization. fixing it:
* `cp github.com/obnoxxx/samba-operator/pkg/apis/smbservice/v1alpha1/zz_generated.deepcopy.go  pkg/apis/smbservice/v1alpha1/`

### Build and run locally

* `operator-sdk build samba-operator`

* `operator-sdk run --local`
* `kubectl apply -f ./mysmbservice.yml`
* `kubectl get smbservice`
* `kubectl describe smbservice my-smbservice`

### Build/push to quay.io and run in kubernetes

* `operator-sdk build quay.io/obnox/samba-operator:v0.0.1`
* `sed -i 's|REPLACE_IMAGE|quay.io/obnox/samba-operator:v0.0.1|g' deploy/operator.yaml`
* `docker push quay.io/obnox/samba-operator:v0.0.1`

* `kubectl create -f deploy/service_account.yaml`
* `kubectl create -f deploy/role.yaml`
* `kubectl create -f deploy/role_binding.yaml`
* `kubectl create -f deploy/operator.yaml`
* `kubectl get deployment`
* `kubectl get pod`

### Implement the mechanism to create smbservice deployments

* edit Reconcile()  in `pkg/controller/smbservice/smbservice_controller.go`
* `operator-sdk build quay.io/obnox/samba-operator:v0.0.1`
* `docker push quay.io/obnox/samba-operator:v0.0.1`
* `kubectl delete pod samba-operator-<TAB>`
* `kubectl apply -f mysmbservice.yml`
* `kubectl get pod` ==> see `my-smbservice-$HASH-$HASH`

### Add new type SmbPvc

* `operator-sdk add api --kind SmbPvc --api-version smbpvc.samba.org/v1alpha1`
* edit `pkg/apis/smbpvc/v1alpha1/smbpvc_types.go`
* `make generate`
* `operator-sdk add controller --kind SmbPvc --api-version smbpvc.samba.org/v1alpha1`
* edit `pkg/controller/smbpvc/smbpvc_controller.go` - adding logic
