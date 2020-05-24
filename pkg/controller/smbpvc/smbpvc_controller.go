package smbpvc

import (
	"context"

	smbpvcv1alpha1 "github.com/obnoxxx/samba-operator/pkg/apis/smbpvc/v1alpha1"
	smbservicev1alpha1 "github.com/obnoxxx/samba-operator/pkg/apis/smbservice/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	// Fetch the SmbPvc instance
	instance := &smbpvcv1alpha1.SmbPvc{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	pvcname := instance.Name + "-pvc"
	svcname := instance.Name + "-svc"

	// create PVC as desired

	// Check if the pvc already exists, if not create a new one
	foundPvc := &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvcname, Namespace: instance.Namespace}, foundPvc)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new pvc
		pvc := r.pvcForSmbPvc(instance, pvcname)
		reqLogger.Info("Creating a new Pvc", "Pvc.Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return reconcile.Result{}, err
		}
		// Pvc created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get PVC")
		return reconcile.Result{}, err
	}

	// create an smbservice on top of the PVC

	foundSvc := &smbservicev1alpha1.SmbService{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: svcname, Namespace: instance.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := r.svcForSmbPvc(instance, svcname, pvcname)
		reqLogger.Info("Creating a new SmbService", "pvc.Name", pvcname)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new SmbService", "svc.Namespace", svc.Namespace, "svc.Name", svc.Name)
			return reconcile.Result{}, err
		}
		// Svc created successfullt - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get PVC")
		return reconcile.Result{}, err
	}

	// all is in shape - don't requeue
	return reconcile.Result{}, nil
}

func (r *ReconcileSmbPvc) pvcForSmbPvc(s *smbpvcv1alpha1.SmbPvc, pvcname string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcname,
			Namespace: s.Namespace,
			//Labels:       pvcLabels,
			//Annotations:  pvcTemplate.Annotations,
		},
		Spec: *s.Spec.Pvc,
	}

	// set the smbpvc instance as the owner and controller
	controllerutil.SetControllerReference(s, pvc, r.scheme)

	return pvc
}

func (r *ReconcileSmbPvc) svcForSmbPvc(s *smbpvcv1alpha1.SmbPvc, svcname string, pvcname string) *smbservicev1alpha1.SmbService {
	svc := &smbservicev1alpha1.SmbService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcname,
			Namespace: s.Namespace,
			//Labels:       pvcLabels,
			//Annotations:  pvcTemplate.Annotations,
		},
		Spec: smbservicev1alpha1.SmbServiceSpec{
			PvcName: pvcname,
		},
	}

	// set the smbpvc instance as the owner and controller
	controllerutil.SetControllerReference(s, svc, r.scheme)

	return svc
}
