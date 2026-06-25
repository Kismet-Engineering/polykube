package controller

import (
	"context"
	"fmt"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterMemberReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ClusterMemberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("clustermember", req.Name)

	var member infrastructure.ClusterMember
	if err := r.Get(ctx, req.NamespacedName, &member); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, &member); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed clustermember", "generation", member.Generation)
	return ctrl.Result{}, nil
}

func (r *ClusterMemberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructure.ClusterMember{}).
		Complete(r)
}

func (r *ClusterMemberReconciler) reconcileStatus(ctx context.Context, member *infrastructure.ClusterMember) error {
	nextStatus := member.Status
	nextStatus.ObservedGeneration = member.Generation
	now := metav1.Now()
	nextStatus.LastObservedAt = &now

	if msg := validateClusterMemberSpec(&member.Spec); msg != "" {
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: member.Generation,
			Reason:             "InvalidSpec",
			Message:            msg,
		})
	} else {
		apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: member.Generation,
			Reason:             "Observed",
			Message:            fmt.Sprintf("ClusterMember %q has been successfully observed.", member.Name),
		})
	}

	if apiequality.Semantic.DeepEqual(member.Status.Conditions, nextStatus.Conditions) &&
		member.Status.ObservedGeneration == nextStatus.ObservedGeneration {
		return nil
	}

	member.Status = nextStatus
	return r.Status().Update(ctx, member)
}

func validateClusterMemberSpec(spec *infrastructure.ClusterMemberSpec) string {
	if spec.ClusterName == "" {
		return "spec.clusterName must not be empty"
	}
	if spec.Provider == "" {
		return "spec.provider must not be empty"
	}
	if spec.Region == "" {
		return "spec.region must not be empty"
	}
	return ""
}
