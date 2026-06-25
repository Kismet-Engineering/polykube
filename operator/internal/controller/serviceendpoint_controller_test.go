package controller

import (
	"context"
	"testing"

	routingapi "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func makeServiceEndpointFixtures(routingMode routingapi.RoutingMode, primaryMemberRef string) (*routingapi.ServiceEndpoint, *runtimeapi.Workload, *corev1.Service) {
	se := &routingapi.ServiceEndpoint{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "echo"},
		Spec: routingapi.ServiceEndpointSpec{
			WorkloadRef:      routingapi.NamespacedObjectReference{Name: "echo"},
			RoutingMode:      routingMode,
			PrimaryMemberRef: primaryMemberRef,
			Hostnames:        []string{"echo.example.com"},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "echo"},
		Spec:       runtimeapi.WorkloadSpec{Image: "hashicorp/http-echo"},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "echo"},
	}
	return se, workload, svc
}

func TestServiceEndpointActiveActiveAnnotations(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	// Re-reconcile after finalizer update triggers another pass.
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var updatedSvc corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updatedSvc); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if got := updatedSvc.Annotations[ciliumGlobalAnnotation]; got != "true" {
		t.Fatalf("cilium global annotation = %q, want true", got)
	}
	if got := updatedSvc.Annotations[ciliumSharedAnnotation]; got != "true" {
		t.Fatalf("cilium shared annotation = %q, want true", got)
	}
}

func TestServiceEndpointActivePassivePrimaryAnnotations(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "alpha")
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint
	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint

	var updatedSvc corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updatedSvc); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if got := updatedSvc.Annotations[ciliumGlobalAnnotation]; got != "true" {
		t.Fatalf("cilium global = %q, want true", got)
	}
	if got := updatedSvc.Annotations[ciliumSharedAnnotation]; got != "true" {
		t.Fatalf("cilium shared = %q, want true (primary cluster)", got)
	}
}

func TestServiceEndpointActivePassiveNonPrimaryAnnotations(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "alpha")
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme:            scheme,
		ClusterMemberName: "beta",
	}

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint
	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint

	var updatedSvc corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updatedSvc); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if got := updatedSvc.Annotations[ciliumGlobalAnnotation]; got != "true" {
		t.Fatalf("cilium global = %q, want true", got)
	}
	if got := updatedSvc.Annotations[ciliumSharedAnnotation]; got != "false" {
		t.Fatalf("cilium shared = %q, want false (non-primary cluster)", got)
	}
}

func TestServiceEndpointDeleteRemovesAnnotations(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	svc.Annotations = map[string]string{
		ciliumGlobalAnnotation: "true",
		ciliumSharedAnnotation: "true",
	}
	// Pre-add finalizer so delete path runs immediately.
	se.Finalizers = []string{serviceEndpointFinalizer}
	now := metav1.Now()
	se.DeletionTimestamp = &now

	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updatedSvc corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updatedSvc); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if _, ok := updatedSvc.Annotations[ciliumGlobalAnnotation]; ok {
		t.Fatalf("cilium global annotation still present after delete")
	}
	if _, ok := updatedSvc.Annotations[ciliumSharedAnnotation]; ok {
		t.Fatalf("cilium shared annotation still present after delete")
	}
}

func TestServiceEndpointStatusFields(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "alpha")
	se.Generation = 5
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint
	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}) //nolint

	var updated routingapi.ServiceEndpoint
	if err := reconciler.Get(context.Background(), client.ObjectKey{Namespace: "demo", Name: "echo"}, &updated); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	if updated.Status.ActiveMemberRef != "alpha" {
		t.Fatalf("ActiveMemberRef = %q, want alpha", updated.Status.ActiveMemberRef)
	}
	if len(updated.Status.ResolvedHostnames) != 1 || updated.Status.ResolvedHostnames[0] != "echo.example.com" {
		t.Fatalf("ResolvedHostnames = %v, want [echo.example.com]", updated.Status.ResolvedHostnames)
	}
	if updated.Status.ObservedGeneration != 5 {
		t.Fatalf("ObservedGeneration = %d, want 5", updated.Status.ObservedGeneration)
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}
}
