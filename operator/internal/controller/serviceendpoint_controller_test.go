package controller

import (
	"context"
	"testing"
	"time"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
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
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "echo", UID: "workload-uid"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "hashicorp/http-echo",
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "demo",
			Name:      "echo",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: runtimeapi.GroupName + "/v1alpha1",
				Kind:       "Workload",
				Name:       "echo",
				UID:        "workload-uid",
				Controller: ptr(true),
			}},
		},
	}
	return se, workload, svc
}

func activePassiveReferences() (*infrastructure.Federation, *infrastructure.ClusterMember) {
	return &infrastructure.Federation{
			ObjectMeta: metav1.ObjectMeta{Name: "primary"},
			Spec: infrastructure.FederationSpec{Members: []infrastructure.FederationMemberReference{
				{Name: "alpha"}, {Name: "beta"},
			}},
		}, &infrastructure.ClusterMember{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
			Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "alpha"},
		}
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
	federation, primary := activePassiveReferences()
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc, federation, primary).WithStatusSubresource(se).Build(),
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
	federation, primary := activePassiveReferences()
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc, federation, primary).WithStatusSubresource(se).Build(),
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

func TestServiceEndpointRequeuesWhenServiceMissing(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter < time.Second {
		t.Fatalf("RequeueAfter = %s, want a positive retry delay", result.RequeueAfter)
	}
	var updated routingapi.ServiceEndpoint
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updated); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Reason != "ServiceNotFound" {
		t.Fatalf("Degraded condition = %#v, want ServiceNotFound", degraded)
	}
	if err := reconciler.Create(context.Background(), svc); err != nil {
		t.Fatalf("Create Service error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("recovery Reconcile() error = %v", err)
	}
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updated); err != nil {
		t.Fatalf("Get recovered ServiceEndpoint error = %v", err)
	}
	if degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded"); degraded != nil {
		t.Fatalf("Degraded condition still present after recovery: %#v", degraded)
	}
	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True after recovery", ready)
	}
}

func TestServiceEndpointDegradedWhenWorkloadMissing(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, _, _ := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertServiceEndpointDegraded(t, reconciler, "WorkloadNotFound")
}

func TestServiceEndpointRefusesUnownedService(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	svc.OwnerReferences = nil
	svc.Annotations = map[string]string{"existing": "keep"}
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertServiceEndpointDegraded(t, reconciler, "ServiceOwnershipConflict")

	var unchanged corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &unchanged); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if _, ok := unchanged.Annotations[ciliumGlobalAnnotation]; ok {
		t.Fatal("unowned Service received Cilium annotations")
	}
	if unchanged.Annotations["existing"] != "keep" {
		t.Fatalf("existing annotations changed: %v", unchanged.Annotations)
	}
}

func TestServiceEndpointDeleteDoesNotModifyUnownedService(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActiveActive, "")
	se.Finalizers = []string{serviceEndpointFinalizer}
	now := metav1.Now()
	se.DeletionTimestamp = &now
	svc.OwnerReferences = nil
	svc.Annotations = map[string]string{
		ciliumGlobalAnnotation: "true",
		ciliumSharedAnnotation: "true",
	}
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var unchanged corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &unchanged); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if unchanged.Annotations[ciliumGlobalAnnotation] != "true" || unchanged.Annotations[ciliumSharedAnnotation] != "true" {
		t.Fatalf("unowned Service annotations were modified during delete: %v", unchanged.Annotations)
	}
}

func TestServiceEndpointRejectsPrimaryOutsideFederation(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "gamma")
	federation, _ := activePassiveReferences()
	gamma := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "gamma"},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "gamma"},
	}
	reconciler := &ServiceEndpointReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc, federation, gamma).WithStatusSubresource(se).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertServiceEndpointDegraded(t, reconciler, "PrimaryMemberNotInFederation")
}

func TestServiceEndpointAcceptsPrimarySelectedByFederation(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "alpha")
	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			MemberSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"region": "west"}},
		},
	}
	primary := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Labels: map[string]string{"region": "west"}},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "alpha"},
	}
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc, federation, primary).WithStatusSubresource(se).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "echo"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var updated routingapi.ServiceEndpoint
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updated); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}
}

func TestServiceEndpointStatusFields(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	se, workload, svc := makeServiceEndpointFixtures(routingapi.RoutingModeActivePassive, "alpha")
	se.Generation = 5
	federation, primary := activePassiveReferences()
	reconciler := &ServiceEndpointReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(se, workload, svc, federation, primary).WithStatusSubresource(se).Build(),
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
	if updated.Status.LastTransitionTime == nil {
		t.Fatal("LastTransitionTime is nil")
	}

	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}
}

func assertServiceEndpointDegraded(t *testing.T, reconciler *ServiceEndpointReconciler, reason string) {
	t.Helper()
	var updated routingapi.ServiceEndpoint
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "echo"}, &updated); err != nil {
		t.Fatalf("Get ServiceEndpoint error = %v", err)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reason {
		t.Fatalf("Degraded condition = %#v, want True/%s", degraded, reason)
	}
	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != reason {
		t.Fatalf("Ready condition = %#v, want False/%s", ready, reason)
	}
}
