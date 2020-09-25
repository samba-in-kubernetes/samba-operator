package smbpvc

import (
	"context"

	"github.com/obnoxxx/samba-operator/internal/resources"
	smbpvcv1alpha1 "github.com/obnoxxx/samba-operator/pkg/apis/smbpvc/v1alpha1"
	smbservicev1alpha1 "github.com/obnoxxx/samba-operator/pkg/apis/smbservice/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_smbpvc")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SmbPvc Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSmbPvc{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("smbpvc-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SmbPvc
	err = c.Watch(&source.Kind{Type: &smbpvcv1alpha1.SmbPvc{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner SmbPvc
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &smbpvcv1alpha1.SmbPvc{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &smbservicev1alpha1.SmbService{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &smbpvcv1alpha1.SmbPvc{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSmbPvc implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSmbPvc{}

// ReconcileSmbPvc reconciles a SmbPvc object
type ReconcileSmbPvc struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SmbPvc object and makes changes based on the state read
// and what is in the SmbPvc.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSmbPvc) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SmbPvc")

	smbPvcManager := resources.NewSmbPvcManager(
		r.client, r.scheme, reqLogger)
	res := smbPvcManager.Update(context.TODO(), request.NamespacedName)
	err := res.Err()
	if res.Requeue() {
		return reconcile.Result{Requeue: true}, err
	}
	return reconcile.Result{}, err
}
