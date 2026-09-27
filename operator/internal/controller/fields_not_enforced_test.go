package controller

import (
	"context"
	"strings"
	"testing"

	dataapi "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	routingapi "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// assertFieldsNotEnforced checks the informational condition. An empty want
// means the condition must be absent.
func assertFieldsNotEnforced(t *testing.T, conditions []metav1.Condition, want ...string) {
	t.Helper()
	cond := apimeta.FindStatusCondition(conditions, fieldsNotEnforcedCondition)
	if len(want) == 0 {
		if cond != nil {
			t.Fatalf("%s condition = %#v, want absent", fieldsNotEnforcedCondition, cond)
		}
		return
	}
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "AcceptedNotEnforced" {
		t.Fatalf("%s condition = %#v, want True/AcceptedNotEnforced", fieldsNotEnforcedCondition, cond)
	}
	for _, field := range want {
		if !strings.Contains(cond.Message, field) {
			t.Fatalf("%s message = %q, want it to name %s", fieldsNotEnforcedCondition, cond.Message, field)
		}
	}
}

func reconcileTwice(t *testing.T, r reconcile.Reconciler, key types.NamespacedName) {
	t.Helper()
	for i := 0; i < 2; i++ {
		if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatalf("Reconcile() pass %d error = %v", i+1, err)
		}
	}
}

func clearAndReconcile(t *testing.T, c client.Client, r reconcile.Reconciler, obj client.Object, clear func()) {
	t.Helper()
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(context.Background(), key, obj); err != nil {
		t.Fatalf("Get error = %v", err)
	}
	clear()
	if err := c.Update(context.Background(), obj); err != nil {
		t.Fatalf("Update error = %v", err)
	}
	reconcileTwice(t, r, key)
	if err := c.Get(context.Background(), key, obj); err != nil {
		t.Fatalf("Get error = %v", err)
	}
}

func TestWorkloadReportsFieldsNotEnforced(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			TargetPolicy:  &runtimeapi.WorkloadTargetPolicy{Strategy: "canary"},
			RolloutRef:    &runtimeapi.RolloutReference{Kind: "Rollout", Name: "api"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build()
	r := &WorkloadReconciler{Client: c, Scheme: scheme}
	key := client.ObjectKeyFromObject(workload)

	reconcileTwice(t, r, key)
	var got runtimeapi.Workload
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.targetPolicy.strategy", "spec.rolloutRef")
	if apimeta.FindStatusCondition(got.Status.Conditions, "RuntimeObjectsApplied") == nil {
		t.Fatalf("RuntimeObjectsApplied missing; unenforced fields must not block reconciliation")
	}

	clearAndReconcile(t, c, r, &got, func() {
		got.Spec.TargetPolicy = nil
		got.Spec.RolloutRef = nil
	})
	assertFieldsNotEnforced(t, got.Status.Conditions)
}

func TestWorkloadReportsFieldsNotEnforcedWhenPending(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       infrastructure.FederationSpec{Members: []infrastructure.FederationMemberReference{{Name: "beta"}}},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			RolloutRef:    &runtimeapi.RolloutReference{Name: "api"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, workload).WithStatusSubresource(workload).Build()
	r := &WorkloadReconciler{Client: c, Scheme: scheme, ClusterMemberName: "alpha"}

	reconcileTwice(t, r, client.ObjectKeyFromObject(workload))
	var got runtimeapi.Workload
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(workload), &got); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if apimeta.FindStatusCondition(got.Status.Conditions, "Pending") == nil {
		t.Fatalf("Pending condition missing")
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.rolloutRef")
}

func TestServiceEndpointReportsFieldsNotEnforced(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	se.Spec.FailoverPolicy = &routingapi.FailoverPolicy{Enabled: true}
	se.Spec.GatewayRef = &routingapi.GatewayReference{Name: "public"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build()
	r := &ServiceEndpointReconciler{Client: c, Scheme: scheme}

	reconcileTwice(t, r, client.ObjectKeyFromObject(se))
	var got routingapi.ServiceEndpoint
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(se), &got); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.failoverPolicy", "spec.gatewayRef")
	if ready := apimeta.FindStatusCondition(got.Status.Conditions, "Ready"); ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}

	clearAndReconcile(t, c, r, &got, func() {
		got.Spec.FailoverPolicy = nil
		got.Spec.GatewayRef = nil
	})
	assertFieldsNotEnforced(t, got.Status.Conditions)
}

func TestServiceEndpointReportsFieldsNotEnforcedWhenDegraded(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	se, _, _ := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	se.Spec.GatewayRef = &routingapi.GatewayReference{Name: "public"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(se).WithStatusSubresource(se).Build()
	r := &ServiceEndpointReconciler{Client: c, Scheme: scheme}

	reconcileTwice(t, r, client.ObjectKeyFromObject(se))
	var got routingapi.ServiceEndpoint
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(se), &got); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	if degraded := apimeta.FindStatusCondition(got.Status.Conditions, "Degraded"); degraded == nil || degraded.Reason != "WorkloadNotFound" {
		t.Fatalf("Degraded condition = %#v, want WorkloadNotFound", degraded)
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.gatewayRef")
}

func TestDatastoreBindingReportsFieldsNotEnforced(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	binding, workload, deployment, secret := makeDatastoreFixtures("app-db", "postgres", dataapi.DatastoreReplicationModeActivePassive)
	binding.Spec.ConflictPolicy = dataapi.DatastoreConflictPolicyLastWriteWins
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment, secret).WithStatusSubresource(binding).Build()
	r := &DatastoreBindingReconciler{Client: c, Scheme: scheme}

	reconcileTwice(t, r, client.ObjectKeyFromObject(binding))
	var got dataapi.DatastoreBinding
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(binding), &got); err != nil {
		t.Fatalf("Get DatastoreBinding error = %v", err)
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.conflictPolicy")
	if ready := apimeta.FindStatusCondition(got.Status.Conditions, "Ready"); ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}

	clearAndReconcile(t, c, r, &got, func() { got.Spec.ConflictPolicy = "" })
	assertFieldsNotEnforced(t, got.Status.Conditions)
}

func TestFederationReportsFieldsNotEnforced(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}
	alpha := readyClusterMember("alpha")
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members:             []infrastructure.FederationMemberReference{{Name: "alpha"}},
			DefaultTargetPolicy: &infrastructure.FederationTargetPolicy{Members: []string{"alpha"}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(federation, alpha).WithStatusSubresource(federation).Build()
	r := &FederationReconciler{Client: c, Scheme: scheme}

	reconcileTwice(t, r, client.ObjectKeyFromObject(federation))
	var got infrastructure.Federation
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(federation), &got); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	assertFieldsNotEnforced(t, got.Status.Conditions, "spec.defaultTargetPolicy")
	if ready := apimeta.FindStatusCondition(got.Status.Conditions, "Ready"); ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}

	clearAndReconcile(t, c, r, &got, func() { got.Spec.DefaultTargetPolicy = nil })
	assertFieldsNotEnforced(t, got.Status.Conditions)
}
