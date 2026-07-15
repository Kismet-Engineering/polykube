package controller

import (
	"context"
	"fmt"
	"time"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	routingapi "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	workload, err := r.resolveWorkload(ctx, &se)
	if err != nil {
		return ctrl.Result{}, err
	}
	if workload == nil {
		return r.reconcileDegraded(ctx, &se, "WorkloadNotFound",
			fmt.Sprintf("Workload %q was not found. Create the Workload or update spec.workloadRef.", se.Spec.WorkloadRef.Name))
	}

	if se.Spec.RoutingMode == routingapi.RoutingModeActivePassive {
		reason, message, err := r.validatePrimaryMember(ctx, &se, workload)
		if err != nil {
			return ctrl.Result{}, err
		}
		if reason != "" {
			return r.reconcileDegraded(ctx, &se, reason, message)
		}
	}

	svc, err := r.resolveService(ctx, workload)
	if err != nil {
		return ctrl.Result{}, err
	}
	if svc == nil {
		return r.reconcileDegraded(ctx, &se, "ServiceNotFound",
			fmt.Sprintf("Service %q was not found in namespace %q. Wait for the Workload to create it or declare a Workload port.", workload.Name, workload.Namespace))
	}
	if !metav1.IsControlledBy(svc, workload) {
		return r.reconcileDegraded(ctx, &se, "ServiceOwnershipConflict",
			fmt.Sprintf("Service %q in namespace %q is not controlled by Workload %q. Refusing to modify its routing annotations.", svc.Name, svc.Namespace, workload.Name))
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

func (r *ServiceEndpointReconciler) resolveWorkload(ctx context.Context, se *routingapi.ServiceEndpoint) (*runtimeapi.Workload, error) {
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
	return &workload, nil
}

func (r *ServiceEndpointReconciler) resolveService(ctx context.Context, workload *runtimeapi.Workload) (*corev1.Service, error) {
	var svc corev1.Service
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: workload.Name}, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}

func (r *ServiceEndpointReconciler) validatePrimaryMember(ctx context.Context, se *routingapi.ServiceEndpoint, workload *runtimeapi.Workload) (string, string, error) {
	var member infrastructure.ClusterMember
	if err := r.Get(ctx, client.ObjectKey{Name: se.Spec.PrimaryMemberRef}, &member); err != nil {
		if apierrors.IsNotFound(err) {
			return "PrimaryMemberNotFound", fmt.Sprintf("ClusterMember %q referenced by spec.primaryMemberRef was not found.", se.Spec.PrimaryMemberRef), nil
		}
		return "", "", err
	}

	federationName := workload.Spec.FederationRef.Name
	if federationName == "" {
		return "PrimaryMemberNotInFederation", fmt.Sprintf("Workload %q does not reference a Federation; cannot use ClusterMember %q as an active/passive primary.", workload.Name, member.Name), nil
	}
	var federation infrastructure.Federation
	if err := r.Get(ctx, client.ObjectKey{Name: federationName}, &federation); err != nil {
		if apierrors.IsNotFound(err) {
			return "FederationNotFound", fmt.Sprintf("Federation %q referenced by Workload %q was not found.", federationName, workload.Name), nil
		}
		return "", "", err
	}

	for _, ref := range federation.Spec.Members {
		if ref.Name == member.Name {
			return "", "", nil
		}
	}
	if federation.Spec.MemberSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(federation.Spec.MemberSelector)
		if err != nil {
			return "InvalidFederationSelector", fmt.Sprintf("Federation %q has an invalid memberSelector: %v.", federationName, err), nil
		}
		if selector.Matches(labels.Set(member.Labels)) {
			return "", "", nil
		}
	}
	return "PrimaryMemberNotInFederation", fmt.Sprintf("ClusterMember %q is not a member of Federation %q referenced by Workload %q.", member.Name, federationName, workload.Name), nil
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

	workload, err := r.resolveWorkload(ctx, se)
	if err != nil {
		return err
	}
	if workload != nil {
		svc, err := r.resolveService(ctx, workload)
		if err != nil {
			return err
		}
		if svc != nil && metav1.IsControlledBy(svc, workload) {
			patch := client.MergeFrom(svc.DeepCopy())
			delete(svc.Annotations, ciliumGlobalAnnotation)
			delete(svc.Annotations, ciliumSharedAnnotation)
			if err := r.Patch(ctx, svc, patch); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	controllerutil.RemoveFinalizer(se, serviceEndpointFinalizer)
	return r.Update(ctx, se)
}

func (r *ServiceEndpointReconciler) reconcileStatus(ctx context.Context, se *routingapi.ServiceEndpoint) error {
	nextStatus := se.Status
	nextStatus.ObservedGeneration = se.Generation
	nextStatus.LastTransitionTime = serviceEndpointTransitionTime(se, metav1.ConditionTrue, "Reconciled")

	if se.Spec.RoutingMode == routingapi.RoutingModeActivePassive {
		nextStatus.ActiveMemberRef = se.Spec.PrimaryMemberRef
	} else {
		nextStatus.ActiveMemberRef = ""
	}

	nextStatus.ResolvedHostnames = se.Spec.Hostnames

	apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Degraded")
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

func (r *ServiceEndpointReconciler) reconcileDegraded(ctx context.Context, se *routingapi.ServiceEndpoint, reason, message string) (ctrl.Result, error) {
	nextStatus := se.Status
	nextStatus.ObservedGeneration = se.Generation
	nextStatus.ActiveMemberRef = ""
	nextStatus.ResolvedHostnames = nil
	nextStatus.LastTransitionTime = serviceEndpointTransitionTime(se, metav1.ConditionFalse, reason)

	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: se.Generation,
		Reason:             reason,
		Message:            message,
	})
	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: se.Generation,
		Reason:             reason,
		Message:            message,
	})

	if !apiequality.Semantic.DeepEqual(se.Status, nextStatus) {
		se.Status = nextStatus
		if err := r.Status().Update(ctx, se); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func serviceEndpointTransitionTime(se *routingapi.ServiceEndpoint, status metav1.ConditionStatus, reason string) *metav1.Time {
	ready := apimeta.FindStatusCondition(se.Status.Conditions, "Ready")
	if ready != nil && ready.Status == status && ready.Reason == reason && se.Status.LastTransitionTime != nil {
		return se.Status.LastTransitionTime.DeepCopy()
	}
	now := metav1.Now()
	return &now
}
