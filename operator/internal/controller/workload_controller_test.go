package controller

import (
	"context"
	"testing"

	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWorkloadReconcileIgnoresMissingWorkload(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "missing"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero result", result)
	}
}

func TestWorkloadReconcileObservesExistingWorkload(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero result", result)
	}
}
