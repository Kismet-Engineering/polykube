package controller

import (
	"context"
	"testing"

	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build(),
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

func TestWorkloadReconcileAppliesDeploymentAndService(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	replicas := int32(2)
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			Replicas:      &replicas,
			Ports: []runtimeapi.ContainerPort{{
				Name:          "http",
				ContainerPort: 8080,
			}},
			Env: []runtimeapi.EnvVar{{Name: "MODE", Value: "test"}},
			EnvFrom: []runtimeapi.EnvFromSource{{
				ConfigMapRef: &runtimeapi.LocalObjectReference{Name: "api-config"},
				SecretRef:    &runtimeapi.LocalObjectReference{Name: "api-secrets"},
			}},
			ImagePullSecrets:   []runtimeapi.LocalObjectReference{{Name: "registry"}},
			ServiceAccountName: "api-runner",
		},
	}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero result", result)
	}

	var deployment appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != replicas {
		t.Fatalf("Deployment replicas = %v, want %d", deployment.Spec.Replicas, replicas)
	}
	if got := deployment.Spec.Template.Spec.Containers[0].Image; got != workload.Spec.Image {
		t.Fatalf("Deployment image = %q, want %q", got, workload.Spec.Image)
	}
	if got := deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort; got != 8080 {
		t.Fatalf("Deployment container port = %d, want 8080", got)
	}
	if got := deployment.Spec.Template.Spec.Containers[0].Env[0].Value; got != "test" {
		t.Fatalf("Deployment env value = %q, want test", got)
	}
	if got := deployment.Spec.Template.Spec.ServiceAccountName; got != "api-runner" {
		t.Fatalf("Deployment service account = %q, want api-runner", got)
	}
	if got := deployment.Spec.Template.Spec.ImagePullSecrets[0].Name; got != "registry" {
		t.Fatalf("Deployment image pull secret = %q, want registry", got)
	}
	if !metav1.IsControlledBy(&deployment, workload) {
		t.Fatalf("Deployment is not controlled by Workload")
	}

	var service corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &service); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
	if got := service.Spec.Ports[0].Port; got != 8080 {
		t.Fatalf("Service port = %d, want 8080", got)
	}
	if got := service.Spec.Selector[workloadNameLabel]; got != "api" {
		t.Fatalf("Service selector workload = %q, want api", got)
	}
	if !metav1.IsControlledBy(&service, workload) {
		t.Fatalf("Service is not controlled by Workload")
	}
}

func TestWorkloadReconcileDeletesServiceWhenNoPortsDeclared(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api", UID: "workload-uid"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Namespace: "demo",
		Name:      "api",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion:         runtimeapi.GroupName + "/v1alpha1",
			Kind:               "Workload",
			Name:               "api",
			UID:                "workload-uid",
			Controller:         ptr(true),
			BlockOwnerDeletion: ptr(true),
		}},
	}}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, service).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var deleted corev1.Service
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deleted)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get Service error = %v, want not found", err)
	}

	var deployment appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
}

func TestWorkloadReconcileKeepsUnownedServiceWhenNoPortsDeclared(t *testing.T) {
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
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"}}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, service).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var existing corev1.Service
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &existing); err != nil {
		t.Fatalf("Get Service error = %v", err)
	}
}

func TestWorkloadReconcileReportsReconcilingStatus(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api", Generation: 3},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v2",
		},
	}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if updated.Status.ObservedGeneration != 3 {
		t.Fatalf("ObservedGeneration = %d, want 3", updated.Status.ObservedGeneration)
	}
	if updated.Status.ActiveImage != "example/api:v2" {
		t.Fatalf("ActiveImage = %q, want example/api:v2", updated.Status.ActiveImage)
	}
	if len(updated.Status.Targets) != 1 {
		t.Fatalf("Targets length = %d, want 1", len(updated.Status.Targets))
	}
	target := updated.Status.Targets[0]
	if target.ClusterMemberRef != localClusterMemberRef {
		t.Fatalf("ClusterMemberRef = %q, want %q", target.ClusterMemberRef, localClusterMemberRef)
	}
	if target.State != runtimeapi.WorkloadTargetStateReconciling {
		t.Fatalf("Target state = %q, want %q", target.State, runtimeapi.WorkloadTargetStateReconciling)
	}
	if target.RuntimeRef != "api" {
		t.Fatalf("RuntimeRef = %q, want api", target.RuntimeRef)
	}
	if target.LastTransitionTime == nil {
		t.Fatalf("LastTransitionTime is nil")
	}

	applied := apimeta.FindStatusCondition(updated.Status.Conditions, "RuntimeObjectsApplied")
	if applied == nil || applied.Status != metav1.ConditionTrue {
		t.Fatalf("RuntimeObjectsApplied condition = %#v, want True", applied)
	}
	available := apimeta.FindStatusCondition(updated.Status.Conditions, "Available")
	if available == nil || available.Status != metav1.ConditionFalse || available.Reason != "DeploymentReconciling" {
		t.Fatalf("Available condition = %#v, want False DeploymentReconciling", available)
	}
}

func TestWorkloadReconcileReportsAvailableStatus(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api", Generation: 4},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v3",
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		}}},
	}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, deployment).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStateAvailable {
		t.Fatalf("Target state = %q, want %q", updated.Status.Targets[0].State, runtimeapi.WorkloadTargetStateAvailable)
	}
	available := apimeta.FindStatusCondition(updated.Status.Conditions, "Available")
	if available == nil || available.Status != metav1.ConditionTrue || available.Reason != "DeploymentAvailable" {
		t.Fatalf("Available condition = %#v, want True DeploymentAvailable", available)
	}
}

func ptr[T any](value T) *T {
	return &value
}
