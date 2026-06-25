package controller

import (
	"context"
	"testing"

	dataapi "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
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

func makeDatastoreFixtures(bindingName, engine string, replicationMode dataapi.DatastoreReplicationMode) (*dataapi.DatastoreBinding, *runtimeapi.Workload, *appsv1.Deployment, *corev1.Secret) {
	binding := &dataapi.DatastoreBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: bindingName},
		Spec: dataapi.DatastoreBindingSpec{
			WorkloadRef:     dataapi.NamespacedObjectReference{Name: "api"},
			Engine:          engine,
			ConnectionRef:   dataapi.NamespacedObjectReference{Name: "db-secret"},
			ReplicationMode: replicationMode,
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec:       runtimeapi.WorkloadSpec{Image: "example/api:v1"},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "example/api:v1"}},
				},
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "db-secret"},
		Data:       map[string][]byte{"url": []byte("postgres://user:pass@db:5433/mydb?sslmode=disable")},
	}
	return binding, workload, deployment, secret
}

func reconcileDatastoreBinding(t *testing.T, reconciler *DatastoreBindingReconciler, ns, name string) {
	t.Helper()
	// Two passes: first adds the finalizer, second does the actual work.
	for i := 0; i < 2; i++ {
		if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}); err != nil {
			t.Fatalf("Reconcile() pass %d error = %v", i+1, err)
		}
	}
}

func TestDatastoreBindingHappyPath(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, workload, deployment, secret := makeDatastoreFixtures("app-db", "yugabytedb", dataapi.DatastoreReplicationModeActiveActive)
	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment, secret).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	reconcileDatastoreBinding(t, reconciler, "demo", "app-db")

	var updatedDeploy appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updatedDeploy); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	env := updatedDeploy.Spec.Template.Spec.Containers[0].Env
	if !hasEnvVar(env, "DATASTORE_APP_DB_URL") {
		t.Fatalf("DATASTORE_APP_DB_URL not injected; env = %v", env)
	}
	if !hasEnvVar(env, "DATASTORE_APP_DB_REPLICATION_MODE") {
		t.Fatalf("DATASTORE_APP_DB_REPLICATION_MODE not injected; env = %v", env)
	}
	if hasEnvVar(env, "DATABASE_URL") {
		t.Fatalf("DATABASE_URL should not be set for non-primary binding")
	}

	var updated dataapi.DatastoreBinding
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "app-db"}, &updated); err != nil {
		t.Fatalf("Get DatastoreBinding error = %v", err)
	}
	ready := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}
}

func TestDatastoreBindingPrimaryInjectsDatabaseURL(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, workload, deployment, secret := makeDatastoreFixtures("primary", "yugabytedb", dataapi.DatastoreReplicationModeActiveActive)
	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment, secret).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	reconcileDatastoreBinding(t, reconciler, "demo", "primary")

	var updatedDeploy appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updatedDeploy); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	env := updatedDeploy.Spec.Template.Spec.Containers[0].Env
	if !hasEnvVar(env, "DATABASE_URL") {
		t.Fatalf("DATABASE_URL not injected for primary binding; env = %v", env)
	}
	if !hasEnvVar(env, "DATASTORE_PRIMARY_URL") {
		t.Fatalf("DATASTORE_PRIMARY_URL not injected; env = %v", env)
	}
}

func TestDatastoreBindingMissingWorkload(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, _, _, secret := makeDatastoreFixtures("app-db", "yugabytedb", dataapi.DatastoreReplicationModeNone)
	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, secret).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	reconcileDatastoreBinding(t, reconciler, "demo", "app-db")

	var updated dataapi.DatastoreBinding
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "app-db"}, &updated); err != nil {
		t.Fatalf("Get DatastoreBinding error = %v", err)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "WorkloadNotFound" {
		t.Fatalf("Degraded condition = %#v, want True/WorkloadNotFound", degraded)
	}
}

func TestDatastoreBindingMissingConnectionSecret(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, workload, deployment, _ := makeDatastoreFixtures("app-db", "yugabytedb", dataapi.DatastoreReplicationModeNone)
	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	reconcileDatastoreBinding(t, reconciler, "demo", "app-db")

	var updated dataapi.DatastoreBinding
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "app-db"}, &updated); err != nil {
		t.Fatalf("Get DatastoreBinding error = %v", err)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "ConnectionSecretNotFound" {
		t.Fatalf("Degraded condition = %#v, want True/ConnectionSecretNotFound", degraded)
	}

	// Deployment must not be mutated.
	var updatedDeploy appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updatedDeploy); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	if len(updatedDeploy.Spec.Template.Spec.Containers[0].Env) != 0 {
		t.Fatalf("Deployment env should not be patched when secret is missing")
	}
}

func TestDatastoreBindingUnsupportedEngine(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, workload, deployment, secret := makeDatastoreFixtures("app-db", "mongodb", dataapi.DatastoreReplicationModeNone)
	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment, secret).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	reconcileDatastoreBinding(t, reconciler, "demo", "app-db")

	var updated dataapi.DatastoreBinding
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "app-db"}, &updated); err != nil {
		t.Fatalf("Get DatastoreBinding error = %v", err)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "UnsupportedEngine" {
		t.Fatalf("Degraded condition = %#v, want True/UnsupportedEngine", degraded)
	}
}

func TestDatastoreBindingDeleteRemovesEnvVars(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	binding, workload, deployment, secret := makeDatastoreFixtures("primary", "yugabytedb", dataapi.DatastoreReplicationModeActiveActive)
	binding.Finalizers = []string{datastoreBindingFinalizer}
	now := metav1.Now()
	binding.DeletionTimestamp = &now

	// Pre-inject env vars so the delete reconcile has something to remove.
	deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "DATABASE_URL", Value: "postgres://..."},
		{Name: "DATASTORE_PRIMARY_URL", Value: "postgres://..."},
		{Name: "DATASTORE_PRIMARY_REPLICATION_MODE", Value: "ActiveActive"},
		{Name: "OTHER_VAR", Value: "keep-me"},
	}

	reconciler := &DatastoreBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, workload, deployment, secret).WithStatusSubresource(binding).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "primary"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updatedDeploy appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updatedDeploy); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	env := updatedDeploy.Spec.Template.Spec.Containers[0].Env
	if hasEnvVar(env, "DATABASE_URL") {
		t.Fatalf("DATABASE_URL still present after binding delete")
	}
	if hasEnvVar(env, "DATASTORE_PRIMARY_URL") {
		t.Fatalf("DATASTORE_PRIMARY_URL still present after binding delete")
	}
	if !hasEnvVar(env, "OTHER_VAR") {
		t.Fatalf("OTHER_VAR was unexpectedly removed")
	}

	// Binding itself should have been released (finalizer removed).
	var updatedBinding dataapi.DatastoreBinding
	getErr := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "primary"}, &updatedBinding)
	if !apierrors.IsNotFound(getErr) {
		// fake client may not run garbage collection, just check finalizer is removed.
		if len(updatedBinding.Finalizers) != 0 {
			t.Fatalf("finalizer not removed after delete")
		}
	}
}

func hasEnvVar(env []corev1.EnvVar, name string) bool {
	for _, e := range env {
		if e.Name == name {
			return true
		}
	}
	return false
}
