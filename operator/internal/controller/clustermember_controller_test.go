package controller

import (
	"context"
	"testing"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterMemberReconcileIgnoresMissing(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	reconciler := &ClusterMemberReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero", result)
	}
}

func TestClusterMemberReconcileValidMember(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	member := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Generation: 1},
		Spec: infrastructure.ClusterMemberSpec{
			Provider:    "kind",
			Region:      "local",
			ClusterName: "alpha",
		},
	}

	reconciler := &ClusterMemberReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(member).WithStatusSubresource(member).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "alpha"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero", result)
	}

	var updated infrastructure.ClusterMember
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "alpha"}, &updated); err != nil {
		t.Fatalf("Get ClusterMember error = %v", err)
	}
	if updated.Status.ObservedGeneration != 1 {
		t.Fatalf("ObservedGeneration = %d, want 1", updated.Status.ObservedGeneration)
	}
	if updated.Status.LastObservedAt == nil {
		t.Fatalf("LastObservedAt is nil")
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != "Observed" {
		t.Fatalf("Ready condition = %#v, want True/Observed", ready)
	}
}

func TestClusterMemberReconcileInvalidSpecMissingClusterName(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	member := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: infrastructure.ClusterMemberSpec{
			Provider: "kind",
			Region:   "local",
			// ClusterName intentionally empty
		},
	}

	reconciler := &ClusterMemberReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(member).WithStatusSubresource(member).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "bad"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.ClusterMember
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "bad"}, &updated); err != nil {
		t.Fatalf("Get ClusterMember error = %v", err)
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != "InvalidSpec" {
		t.Fatalf("Ready condition = %#v, want False/InvalidSpec", ready)
	}
}

func TestClusterMemberReconcileInvalidSpecMissingProvider(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	member := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: infrastructure.ClusterMemberSpec{
			ClusterName: "alpha",
			Region:      "local",
			// Provider intentionally empty
		},
	}

	reconciler := &ClusterMemberReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(member).WithStatusSubresource(member).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "bad"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.ClusterMember
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "bad"}, &updated); err != nil {
		t.Fatalf("Get ClusterMember error = %v", err)
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != "InvalidSpec" {
		t.Fatalf("Ready condition = %#v, want False/InvalidSpec", ready)
	}
}
