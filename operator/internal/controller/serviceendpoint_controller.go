package controller

import (
	"context"
	"time"

	routingapi "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ciliumGlobalAnnotation   = "service.cilium.io/global"
	ciliumSharedAnnotation   = "service.cilium.io/shared"
	serviceEndpointFinalizer = "serviceendpoint.routing.polykube.dev/finalizer"
)

type ServiceEndpointReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ClusterMemberName string
}

func (r *ServiceEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("serviceendpoint", req.NamespacedName.String())

	var se routingapi.ServiceEndpoint
	if err := r.Get(ctx, req.NamespacedName, &se); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !se.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, &se)
	}

	if controllerutil.AddFinalizer(&se, serviceEndpointFinalizer) {
		if err := r.Update(ctx, &se); err != nil {
			return ctrl.Result{}, err
		}
	}

	svc, err := r.resolveService(ctx, &se)
	if err != nil {
		return ctrl.Result{}, err
	}
	if svc == nil {
		logger.Info("service not found, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.applyAnnotations(ctx, &se, svc); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, &se); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed serviceendpoint", "generation", se.Generation)
	return ctrl.Result{}, nil
}

func (r *ServiceEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&routingapi.ServiceEndpoint{}).
		Complete(r)
}

func (r *ServiceEndpointReconciler) resolveService(ctx context.Context, se *routingapi.ServiceEndpoint) (*corev1.Service, error) {
	// Resolve the Workload to get its namespace.
	workloadNS := se.Spec.WorkloadRef.Namespace
	if workloadNS == "" {
		workloadNS = se.Namespace
	}

	var workload runtimeapi.Workload
	if err := r.Get(ctx, client.ObjectKey{Namespace: workloadNS, Name: se.Spec.WorkloadRef.Name}, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	// The Service owned by the Workload has the same name and namespace.
	var svc corev1.Service
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: workload.Name}, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}

func (r *ServiceEndpointReconciler) applyAnnotations(ctx context.Context, se *routingapi.ServiceEndpoint, svc *corev1.Service) error {
	patch := client.MergeFrom(svc.DeepCopy())

	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}

	switch se.Spec.RoutingMode {
	case routingapi.RoutingModeActiveActive:
		svc.Annotations[ciliumGlobalAnnotation] = "true"
		svc.Annotations[ciliumSharedAnnotation] = "true"
	case routingapi.RoutingModeActivePassive:
		svc.Annotations[ciliumGlobalAnnotation] = "true"
		if r.ClusterMemberName == se.Spec.PrimaryMemberRef {
			svc.Annotations[ciliumSharedAnnotation] = "true"
		} else {
			svc.Annotations[ciliumSharedAnnotation] = "false"
		}
	}

	return r.Patch(ctx, svc, patch)
}

func (r *ServiceEndpointReconciler) reconcileDelete(ctx context.Context, se *routingapi.ServiceEndpoint) error {
	if !controllerutil.ContainsFinalizer(se, serviceEndpointFinalizer) {
		return nil
	}

	svc, err := r.resolveService(ctx, se)
	if err != nil {
		return err
	}
	if svc != nil {
		patch := client.MergeFrom(svc.DeepCopy())
		delete(svc.Annotations, ciliumGlobalAnnotation)
		delete(svc.Annotations, ciliumSharedAnnotation)
		if err := r.Patch(ctx, svc, patch); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	controllerutil.RemoveFinalizer(se, serviceEndpointFinalizer)
	return r.Update(ctx, se)
}

func (r *ServiceEndpointReconciler) reconcileStatus(ctx context.Context, se *routingapi.ServiceEndpoint) error {
	nextStatus := se.Status
	nextStatus.ObservedGeneration = se.Generation

	if se.Spec.RoutingMode == routingapi.RoutingModeActivePassive {
		nextStatus.ActiveMemberRef = se.Spec.PrimaryMemberRef
	} else {
		nextStatus.ActiveMemberRef = ""
	}

	nextStatus.ResolvedHostnames = se.Spec.Hostnames

	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: se.Generation,
		Reason:             "Reconciled",
		Message:            "Cilium annotations applied to the Service.",
	})

	if apiequality.Semantic.DeepEqual(se.Status, nextStatus) {
		return nil
	}
	se.Status = nextStatus
	return r.Status().Update(ctx, se)
}
