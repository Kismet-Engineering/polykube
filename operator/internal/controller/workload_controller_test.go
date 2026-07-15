package controller

import (
	"context"
	"testing"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
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

	pullSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "registry"}}
	envSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api-secrets"}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api-config"}}

	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, pullSecret, envSecret, configMap).WithStatusSubresource(workload).Build(),
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
	if target.ClusterMemberRef != "local" {
		t.Fatalf("ClusterMemberRef = %q, want local", target.ClusterMemberRef)
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
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api", UID: "workload-uid", Generation: 4},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v3",
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "demo",
			Name:      "api",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: runtimeapi.GroupName + "/v1alpha1",
				Kind:       "Workload",
				Name:       "api",
				UID:        "workload-uid",
				Controller: ptr(true),
			}},
		},
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

func TestWorkloadReconcileFederationMemberUsesRealMemberRef(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}

	reconciler := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("Reconcile() result = %#v, want zero", result)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 {
		t.Fatalf("Targets length = %d, want 1", len(updated.Status.Targets))
	}
	if got := updated.Status.Targets[0].ClusterMemberRef; got != "alpha" {
		t.Fatalf("ClusterMemberRef = %q, want alpha", got)
	}
}

func TestWorkloadReconcileNotFederationMemberSetsPending(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}

	reconciler := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "gamma",
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatalf("Reconcile() result = %#v, want retry", result)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStatePending {
		t.Fatalf("Target state = %v, want Pending", updated.Status.Targets)
	}

	var deployment appsv1.Deployment
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Deployment should not be created for non-member, got err = %v", err)
	}
}

func TestWorkloadReconcileTargetPolicyMembersFilter(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			TargetPolicy:  &runtimeapi.WorkloadTargetPolicy{Members: []string{"alpha"}},
		},
	}

	reconciler := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "beta",
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStatePending {
		t.Fatalf("Target state = %v, want Pending (excluded by targetPolicy.members)", updated.Status.Targets)
	}
}

func TestWorkloadReconcileRecoversAfterFederationMembershipChange(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "beta"}},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation).WithStatusSubresource(workload).Build()
	reconciler := &WorkloadReconciler{Client: fakeClient, Scheme: scheme, ClusterMemberName: "alpha"}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}

	result, err := reconciler.Reconcile(context.Background(), request)
	if err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("first Reconcile() should retry pending membership")
	}

	var updatedFederation infrastructure.Federation
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "primary"}, &updatedFederation); err != nil {
		t.Fatalf("Get Federation error = %v", err)
	}
	updatedFederation.Spec.Members = append(updatedFederation.Spec.Members, infrastructure.FederationMemberReference{Name: "alpha"})
	if err := fakeClient.Update(context.Background(), &updatedFederation); err != nil {
		t.Fatalf("Update Federation error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var updatedWorkload runtimeapi.Workload
	if err := fakeClient.Get(context.Background(), request.NamespacedName, &updatedWorkload); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if pending := apimeta.FindStatusCondition(updatedWorkload.Status.Conditions, "Pending"); pending != nil {
		t.Fatalf("Pending condition still present after recovery: %#v", pending)
	}
	var deployment appsv1.Deployment
	if err := fakeClient.Get(context.Background(), request.NamespacedName, &deployment); err != nil {
		t.Fatalf("Deployment should exist after membership recovery, got err = %v", err)
	}
}

func TestWorkloadReconcileTargetPolicyMemberSelectorFilter(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}, {Name: "beta"}},
		},
	}
	// alpha has env=prod; the workload only targets env=prod
	alphaMember := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Labels: map[string]string{"env": "prod"}},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "alpha"},
	}
	betaMember := &infrastructure.ClusterMember{
		ObjectMeta: metav1.ObjectMeta{Name: "beta", Labels: map[string]string{"env": "dev"}},
		Spec:       infrastructure.ClusterMemberSpec{Provider: "kind", Region: "local", ClusterName: "beta"},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			TargetPolicy: &runtimeapi.WorkloadTargetPolicy{
				MemberSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			},
		},
	}

	// beta (env=dev) is excluded by the selector
	reconcilerBeta := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation, alphaMember, betaMember).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "beta",
	}

	if _, err := reconcilerBeta.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var updated runtimeapi.Workload
	if err := reconcilerBeta.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStatePending {
		t.Fatalf("Target state = %v, want Pending (excluded by targetPolicy.memberSelector)", updated.Status.Targets)
	}
}

func TestWorkloadReconcileDegradedOnMissingFederation(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "missing"},
			Image:         "example/api:v1",
		},
	}
	reconciler := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("Reconcile() should requeue while Federation is missing")
	}

	assertWorkloadDegraded(t, reconciler, "FederationNotFound")
}

func TestWorkloadReconcileDegradedOnInvalidTargetPolicy(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	federation := &infrastructure.Federation{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec: infrastructure.FederationSpec{
			Members: []infrastructure.FederationMemberReference{{Name: "alpha"}},
		},
	}
	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			TargetPolicy: &runtimeapi.WorkloadTargetPolicy{MemberSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "env", Operator: metav1.LabelSelectorOperator("Invalid")}},
			}},
		},
	}
	reconciler := &WorkloadReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, federation).WithStatusSubresource(workload).Build(),
		Scheme:            scheme,
		ClusterMemberName: "alpha",
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertWorkloadDegraded(t, reconciler, "InvalidTargetPolicy")
}

func TestWorkloadReconcileDegradedOnMissingImagePullSecret(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef:    runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:            "example/api:v1",
			ImagePullSecrets: []runtimeapi.LocalObjectReference{{Name: "registry"}},
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
	if result.RequeueAfter == 0 {
		t.Fatalf("Reconcile() result.RequeueAfter = 0, want non-zero requeue")
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStateDegraded {
		t.Fatalf("Target state = %v, want Degraded", updated.Status.Targets)
	}

	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "SecretNotFound" {
		t.Fatalf("Degraded condition = %#v, want True/SecretNotFound", degraded)
	}

	var deployment appsv1.Deployment
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Deployment should not exist while secret is missing, got err = %v", err)
	}
}

func TestWorkloadReconcileDegradedOnMissingEnvFromSecret(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef: runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:         "example/api:v1",
			EnvFrom: []runtimeapi.EnvFromSource{{
				SecretRef: &runtimeapi.LocalObjectReference{Name: "api-secrets"},
			}},
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
	if result.RequeueAfter == 0 {
		t.Fatalf("Reconcile() result.RequeueAfter = 0, want non-zero requeue")
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStateDegraded {
		t.Fatalf("Target state = %v, want Degraded", updated.Status.Targets)
	}

	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "SecretNotFound" {
		t.Fatalf("Degraded condition = %#v, want True/SecretNotFound", degraded)
	}

	var deployment appsv1.Deployment
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Deployment should not exist while secret is missing, got err = %v", err)
	}
}

func TestWorkloadReconcileDegradedOnMissingConfigMap(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			Image: "example/api:v1",
			EnvFrom: []runtimeapi.EnvFromSource{{
				ConfigMapRef: &runtimeapi.LocalObjectReference{Name: "api-config"},
			}},
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
	if result.RequeueAfter == 0 {
		t.Fatal("Reconcile() should requeue while ConfigMap is missing")
	}
	assertWorkloadDegraded(t, reconciler, "ConfigMapNotFound")

	var deployment appsv1.Deployment
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Deployment should not exist while ConfigMap is missing, got err = %v", err)
	}
}

func TestWorkloadReconcileDegradedOnDeploymentOwnershipConflict(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec:       runtimeapi.WorkloadSpec{Image: "example/api:v2"},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "existing", Image: "example/other:v1"}},
		}}},
	}
	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, deployment).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertWorkloadDegraded(t, reconciler, "DeploymentOwnershipConflict")

	var unchanged appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &unchanged); err != nil {
		t.Fatalf("Get Deployment error = %v", err)
	}
	if got := unchanged.Spec.Template.Spec.Containers[0].Image; got != "example/other:v1" {
		t.Fatalf("conflicting Deployment image = %q, want unchanged", got)
	}
}

func TestWorkloadReconcileDegradedOnServiceOwnershipConflict(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			Image: "example/api:v1",
			Ports: []runtimeapi.ContainerPort{{Name: "http", ContainerPort: 8080}},
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "other"}},
	}
	reconciler := &WorkloadReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload, service).WithStatusSubresource(workload).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertWorkloadDegraded(t, reconciler, "ServiceOwnershipConflict")

	var deployment appsv1.Deployment
	err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Deployment should not be created before Service conflict is resolved, got err = %v", err)
	}
}

func TestWorkloadReconcileRecoverFromMissingSecret(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			FederationRef:    runtimeapi.NamespacedObjectReference{Name: "primary"},
			Image:            "example/api:v1",
			ImagePullSecrets: []runtimeapi.LocalObjectReference{{Name: "registry"}},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build()
	reconciler := &WorkloadReconciler{Client: fakeClient, Scheme: scheme}

	// First reconcile: secret missing → Degraded, no Deployment.
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatalf("first Reconcile() should requeue")
	}

	// Secret appears.
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "registry"}}
	if err := fakeClient.Create(context.Background(), secret); err != nil {
		t.Fatalf("Create Secret error = %v", err)
	}

	// Second reconcile: secret present → Deployment created, Degraded condition cleared.
	result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}})
	if err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("second Reconcile() should not requeue, got %v", result.RequeueAfter)
	}

	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded"); degraded != nil {
		t.Fatalf("Degraded condition still present after recovery: %#v", degraded)
	}

	var deployment appsv1.Deployment
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &deployment); err != nil {
		t.Fatalf("Deployment should exist after recovery, got err = %v", err)
	}
}

func TestWorkloadReconcileRecoverFromMissingConfigMap(t *testing.T) {
	scheme, err := polykubescheme.New()
	if err != nil {
		t.Fatalf("scheme.New() error = %v", err)
	}

	workload := &runtimeapi.Workload{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api"},
		Spec: runtimeapi.WorkloadSpec{
			Image: "example/api:v1",
			EnvFrom: []runtimeapi.EnvFromSource{{
				ConfigMapRef: &runtimeapi.LocalObjectReference{Name: "api-config"},
			}},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workload).WithStatusSubresource(workload).Build()
	reconciler := &WorkloadReconciler{Client: fakeClient, Scheme: scheme}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "demo", Name: "api"}}

	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api-config"}}
	if err := fakeClient.Create(context.Background(), configMap); err != nil {
		t.Fatalf("Create ConfigMap error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	var updated runtimeapi.Workload
	if err := fakeClient.Get(context.Background(), request.NamespacedName, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded"); degraded != nil {
		t.Fatalf("Degraded condition still present after recovery: %#v", degraded)
	}
	var deployment appsv1.Deployment
	if err := fakeClient.Get(context.Background(), request.NamespacedName, &deployment); err != nil {
		t.Fatalf("Deployment should exist after recovery, got err = %v", err)
	}
}

func assertWorkloadDegraded(t *testing.T, reconciler *WorkloadReconciler, reason string) {
	t.Helper()
	var updated runtimeapi.Workload
	if err := reconciler.Get(context.Background(), types.NamespacedName{Namespace: "demo", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get Workload error = %v", err)
	}
	if len(updated.Status.Targets) != 1 || updated.Status.Targets[0].State != runtimeapi.WorkloadTargetStateDegraded {
		t.Fatalf("Target state = %v, want Degraded", updated.Status.Targets)
	}
	degraded := apimeta.FindStatusCondition(updated.Status.Conditions, "Degraded")
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reason {
		t.Fatalf("Degraded condition = %#v, want True/%s", degraded, reason)
	}
}

func ptr[T any](value T) *T {
	return &value
}
