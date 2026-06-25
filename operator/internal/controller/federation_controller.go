package controller

import (
	"context"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type FederationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FederationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("federation", req.Name)

	var federation infrastructure.Federation
	if err := r.Get(ctx, req.NamespacedName, &federation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	members, err := r.resolveMembers(ctx, &federation)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, &federation, members); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed federation", "generation", federation.Generation, "members", len(members))
	return ctrl.Result{}, nil
}

func (r *FederationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// When any ClusterMember changes, re-evaluate all Federations.
	enqueueAllFederations := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		var list infrastructure.FederationList
		if err := mgr.GetClient().List(ctx, &list); err != nil {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(list.Items))
		for _, f := range list.Items {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&f)})
		}
		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructure.Federation{}).
		Watches(&infrastructure.ClusterMember{}, enqueueAllFederations).
		Complete(r)
}

func (r *FederationReconciler) resolveMembers(ctx context.Context, federation *infrastructure.Federation) ([]infrastructure.ClusterMember, error) {
	seen := map[string]struct{}{}
	var members []infrastructure.ClusterMember

	if federation.Spec.MemberSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(federation.Spec.MemberSelector)
		if err != nil {
			return nil, err
		}
		var list infrastructure.ClusterMemberList
		if err := r.List(ctx, &list, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
		for _, m := range list.Items {
			if _, ok := seen[m.Name]; !ok {
				seen[m.Name] = struct{}{}
				members = append(members, m)
			}
		}
	}

	for _, ref := range federation.Spec.Members {
		if _, ok := seen[ref.Name]; ok {
			continue
		}
		var member infrastructure.ClusterMember
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name}, &member); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		seen[member.Name] = struct{}{}
		members = append(members, member)
	}

	return members, nil
}

func (r *FederationReconciler) reconcileStatus(ctx context.Context, federation *infrastructure.Federation, members []infrastructure.ClusterMember) error {
	nextStatus := federation.Status
	nextStatus.ObservedGeneration = federation.Generation

	memberStatuses := make([]infrastructure.FederationMemberStatus, 0, len(members))
	readyCount := int32(0)
	for _, m := range members {
		ready := clusterMemberReady(&m)
		if ready {
			readyCount++
		}
		memberStatuses = append(memberStatuses, infrastructure.FederationMemberStatus{
			Name:  m.Name,
			Ready: ready,
		})
	}
	nextStatus.Members = memberStatuses
	nextStatus.ReadyMembers = readyCount

	allReady := len(members) > 0 && readyCount == int32(len(members))
	if allReady {
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: federation.Generation,
			Reason:             "AllMembersReady",
			Message:            "All resolved ClusterMember resources are Ready.",
		})
		apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Degraded")
	} else if len(members) == 0 {
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: federation.Generation,
			Reason:             "NoMembers",
			Message:            "No ClusterMember resources resolved for this Federation.",
		})
		apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Degraded")
	} else {
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: federation.Generation,
			Reason:             "MembersNotReady",
			Message:            "One or more ClusterMember resources are not Ready.",
		})
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Degraded",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: federation.Generation,
			Reason:             "MembersNotReady",
			Message:            "One or more ClusterMember resources are not Ready.",
		})
	}

	if apiequality.Semantic.DeepEqual(federation.Status, nextStatus) {
		return nil
	}
	federation.Status = nextStatus
	return r.Status().Update(ctx, federation)
}

func clusterMemberReady(member *infrastructure.ClusterMember) bool {
	cond := apimeta.FindStatusCondition(member.Status.Conditions, "Ready")
	return cond != nil && cond.Status == metav1.ConditionTrue
}
