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

func readyClusterMember(name string) *infrastructure.ClusterMember {
	return &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"env": "dev"}},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: name},
		Status: infrastructure.ClusterMemberStatus{
			Conditions: []metav1.Condition{{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "Observed",
			}},
		},
	}
}

func notReadyClusterMember(name string) *infrastructure.ClusterMember {
	return &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"env": "dev"}},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: name},
	}
}

func TestFederationReconcileIgnoresMissing(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	reconciler := &FederationReconciler{
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

func TestFederationReconcileExplicitMembersTwoReady(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	alpha := readyClusterMember("alpha")
	beta := readyClusterMember("beta")
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary", Generation: 2},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}

	reconciler := &FederationReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, alpha, beta).WithStatusSubresource(federation).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.Federation
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "primary"}, &updated); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	if updated.Status.ReadyMembers != 2 {
		t.Fatalf("ReadyMembers = %d, want 2", updated.Status.ReadyMembers)
	}
	if len(updated.Status.Members) != 2 {
		t.Fatalf("Members length = %d, want 2", len(updated.Status.Members))
	}
	if updated.Status.ObservedGeneration != 2 {
		t.Fatalf("ObservedGeneration = %d, want 2", updated.Status.ObservedGeneration)
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != "AllMembersReady" {
		t.Fatalf("Ready condition = %#v, want True/AllMembersReady", ready)
	}
}

func TestFederationReconcileDegradedWhenMemberNotReady(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	alpha := readyClusterMember("alpha")
	beta := notReadyClusterMember("beta")
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}

	reconciler := &FederationReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, alpha, beta).WithStatusSubresource(federation).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.Federation
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "primary"}, &updated); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	if updated.Status.ReadyMembers != 1 {
		t.Fatalf("ReadyMembers = %d, want 1", updated.Status.ReadyMembers)
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionFalse {
		t.Fatalf("Ready condition = %#v, want False", ready)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "MembersNotReady" {
		t.Fatalf("Degraded condition = %#v, want True/MembersNotReady", degraded)
	}
}

func TestFederationReconcileMemberSelector(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	alpha := readyClusterMember("alpha")
	beta := readyClusterMember("beta")
	// gamma has no "env: dev" label
	gamma := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "gamma"},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "gamma"},
	}
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			MemberSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
		},
	}

	reconciler := &FederationReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, alpha, beta, gamma).WithStatusSubresource(federation).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.Federation
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "primary"}, &updated); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	if updated.Status.ReadyMembers != 2 {
		t.Fatalf("ReadyMembers = %d, want 2 (gamma excluded by selector)", updated.Status.ReadyMembers)
	}
	if len(updated.Status.Members) != 2 {
		t.Fatalf("Members length = %d, want 2", len(updated.Status.Members))
	}
}

func TestFederationReconcileSelectorAndExplicitUnion(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	alpha := readyClusterMember("alpha")
	// gamma has no "env: dev" label but is listed explicitly
	gamma := readyClusterMember("gamma")
	gamma.Labels = map[string]string{}
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			MemberSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
			Members:        []infrastructure.FederationMemberReference{{Name: "gamma"}},
		},
	}

	reconciler := &FederationReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, alpha, gamma).WithStatusSubresource(federation).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated infrastructure.Federation
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: "primary"}, &updated); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	// alpha via selector, gamma via explicit list → 2 members
	if len(updated.Status.Members) != 2 {
		t.Fatalf("Members length = %d, want 2 (union of selector and explicit list)", len(updated.Status.Members))
	}
}
