## Followed docs

https://opensource.com/article/20/3/kubernetes-operator-sdk

https://sdk.operatorframework.io/docs/golang/quickstart/


## Steps

* operator-sdk new samba-operator --type go --repo github.com/obnoxxx/samba-operator
* cd samba-operator
* operator-sdk add api --kind SmbService --api-version smbservice.samba.org/v1alpha1
* edit pkg/apis/presentation/v1alpha1/smbservice_types.go, adding pvcname
* operator-sdk generate crds
* operator-sdk generate k8s
* operator-sdk add controller --kind SmbService --api-version smbservice.samba.org/v1alpha1
* kubectl apply -f deploy/crds/smbservice.samba.org_smbservice_crd.yaml

# there was a weirdness in the initialization. fixing it:
* cp github.com/obnoxxx/samba-operator/pkg/apis/smbservice/v1alpha1/zz_generated.deepcopy.go  pkg/apis/smbservice/v1alpha1/

* operator-sdk build samba-operator

* operator-sdk run --local
* kubectl apply -f ./mysmbservice.yml
* kubectl get smbservice
* kubectl describe smbservice my-smbservice

