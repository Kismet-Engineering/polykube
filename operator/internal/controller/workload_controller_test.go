package controller

import (
	"context"
	"testing"

	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"}}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, service).Build(),
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
