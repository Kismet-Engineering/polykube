package controller

import (
	"context"

	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type WorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workload", req.NamespacedName.String())

	var workload runtimeapi.Workload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("observed workload", "generation", workload.Generation)
	return ctrl.Result{}, nil
}

func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimeapi.Workload{}).
		Complete(r)
}
