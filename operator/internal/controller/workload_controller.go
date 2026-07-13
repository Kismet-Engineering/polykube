package controller

import (
	"context"
	"fmt"
	"time"

	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const workloadNameLabel = "polykube.dev/workload"

type WorkloadReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ClusterMemberName string
}

// clusterMemberRef returns the ClusterMember name for this operator instance,
// falling back to "local" when the flag is not set (e.g. in tests or single-cluster mode).
func (r *WorkloadReconciler) clusterMemberRef() string {
	if r.ClusterMemberName != "" {
		return r.ClusterMemberName
	}
	return "local"
}

func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workload", req.NamespacedName.String())

	var workload runtimeapi.Workload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.ClusterMemberName != "" {
		member, err := r.isFederationMember(ctx, &workload)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !member {
			if err := r.reconcilePendingStatus(ctx, &workload, "NotAFederationMember",
				fmt.Sprintf("ClusterMember %q is not a member of Federation %q.", r.ClusterMemberName, workload.Spec.FederationRef.Name)); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		targeted, err := r.isTargetPolicyMatch(ctx, &workload)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !targeted {
			if err := r.reconcilePendingStatus(ctx, &workload, "ExcludedByTargetPolicy",
				fmt.Sprintf("ClusterMember %q is excluded by Workload targetPolicy.", r.ClusterMemberName)); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	missingSecret, err := r.reconcileSecretPreflight(ctx, &workload)
	if err != nil {
		return ctrl.Result{}, err
	}
	if missingSecret != "" {
		if err := r.reconcileDegradedStatus(ctx, &workload, missingSecret); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.reconcileDeployment(ctx, &workload); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, &workload); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileStatus(ctx, &workload); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed workload", "generation", workload.Generation)
	return ctrl.Result{}, nil
}

// reconcileSecretPreflight checks that all secrets referenced by the Workload exist in its namespace.
// Returns the name of the first missing secret, or "" if all are present.
func (r *WorkloadReconciler) reconcileSecretPreflight(ctx context.Context, workload *runtimeapi.Workload) (string, error) {
	for _, ref := range workload.Spec.ImagePullSecrets {
		var secret corev1.Secret
		if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: ref.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return ref.Name, nil
			}
			return "", err
		}
	}
	for _, source := range workload.Spec.EnvFrom {
		if source.SecretRef == nil {
			continue
		}
		var secret corev1.Secret
		if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: source.SecretRef.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return source.SecretRef.Name, nil
			}
			return "", err
		}
	}
	return "", nil
}

func (r *WorkloadReconciler) reconcileDegradedStatus(ctx context.Context, workload *runtimeapi.Workload, missingSecret string) error {
	nextStatus := workload.Status
	nextStatus.ObservedGeneration = workload.Generation

	now := metav1.Now()
	for _, target := range workload.Status.Targets {
		if target.ClusterMemberRef == r.clusterMemberRef() && target.State == runtimeapi.WorkloadTargetStateDegraded && target.LastTransitionTime != nil {
			now = *target.LastTransitionTime
			break
		}
	}
	nextStatus.Targets = []runtimeapi.WorkloadTargetStatus{{
		ClusterMemberRef:   r.clusterMemberRef(),
		State:              runtimeapi.WorkloadTargetStateDegraded,
		LastTransitionTime: &now,
		Message:            fmt.Sprintf("Secret %q not found in namespace %q.", missingSecret, workload.Namespace),
	}}

	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: workload.Generation,
		Reason:             "SecretNotFound",
		Message:            fmt.Sprintf("Secret %q not found in namespace %q. Ensure the secret exists before the Workload is reconciled. See docs/architecture.md for the recommended secrets provisioning model.", missingSecret, workload.Namespace),
	})

	if apiequality.Semantic.DeepEqual(workload.Status, nextStatus) {
		return nil
	}
	workload.Status = nextStatus
	return r.Status().Update(ctx, workload)
}

func (r *WorkloadReconciler) isFederationMember(ctx context.Context, workload *runtimeapi.Workload) (bool, error) {
	federationName := workload.Spec.FederationRef.Name
	if federationName == "" {
		return true, nil
	}

	var federation infrastructure.Federation
	if err := r.Get(ctx, client.ObjectKey{Name: federationName}, &federation); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	for _, ref := range federation.Spec.Members {
		if ref.Name == r.ClusterMemberName {
			return true, nil
		}
	}

	if federation.Spec.MemberSelector != nil {
		var member infrastructure.ClusterMember
		if err := r.Get(ctx, client.ObjectKey{Name: r.ClusterMemberName}, &member); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		selector, err := metav1.LabelSelectorAsSelector(federation.Spec.MemberSelector)
		if err != nil {
			return false, err
		}
		if selector.Matches(labels.Set(member.Labels)) {
			return true, nil
		}
	}

	return false, nil
}

func (r *WorkloadReconciler) isTargetPolicyMatch(ctx context.Context, workload *runtimeapi.Workload) (bool, error) {
	if workload.Spec.TargetPolicy == nil {
		return true, nil
	}
	policy := workload.Spec.TargetPolicy

	if len(policy.Members) > 0 {
		for _, name := range policy.Members {
			if name == r.ClusterMemberName {
				return true, nil
			}
		}
		return false, nil
	}

	if policy.MemberSelector != nil {
		var member infrastructure.ClusterMember
		if err := r.Get(ctx, client.ObjectKey{Name: r.ClusterMemberName}, &member); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		selector, err := metav1.LabelSelectorAsSelector(policy.MemberSelector)
		if err != nil {
			return false, err
		}
		return selector.Matches(labels.Set(member.Labels)), nil
	}

	return true, nil
}

func (r *WorkloadReconciler) reconcilePendingStatus(ctx context.Context, workload *runtimeapi.Workload, reason, message string) error {
	nextStatus := workload.Status
	nextStatus.ObservedGeneration = workload.Generation

	now := metav1.Now()
	for _, target := range workload.Status.Targets {
		if target.ClusterMemberRef == r.clusterMemberRef() && target.State == runtimeapi.WorkloadTargetStatePending && target.LastTransitionTime != nil {
			now = *target.LastTransitionTime
			break
		}
	}
	nextStatus.Targets = []runtimeapi.WorkloadTargetStatus{{
		ClusterMemberRef:   r.clusterMemberRef(),
		State:              runtimeapi.WorkloadTargetStatePending,
		LastTransitionTime: &now,
		Message:            message,
	}}

	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Pending",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: workload.Generation,
		Reason:             reason,
		Message:            message,
	})

	if apiequality.Semantic.DeepEqual(workload.Status, nextStatus) {
		return nil
	}
	workload.Status = nextStatus
	return r.Status().Update(ctx, workload)
}

func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimeapi.Workload{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func (r *WorkloadReconciler) reconcileDeployment(ctx context.Context, workload *runtimeapi.Workload) error {
	deployment := &appsv1.Deployment{}
	deployment.Name = workload.Name
	deployment.Namespace = workload.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := controllerutil.SetControllerReference(workload, deployment, r.Scheme); err != nil {
			return err
		}

		labels := workloadLabels(workload)
		deployment.Labels = labels
		deployment.Spec.Replicas = workload.Spec.Replicas
		deployment.Spec.Selector = metav1LabelSelector(labels)
		deployment.Spec.Template.Labels = labels
		deployment.Spec.Template.Spec.ServiceAccountName = workload.Spec.ServiceAccountName
		deployment.Spec.Template.Spec.ImagePullSecrets = imagePullSecrets(workload.Spec.ImagePullSecrets)
		env := envVars(workload.Spec.Env)
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			env = preserveDatastoreManagedEnvVars(env, deployment.Spec.Template.Spec.Containers[0].Env, datastoreManagedEnvVars(deployment))
		}

		deployment.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:    "app",
			Image:   workload.Spec.Image,
			Ports:   containerPorts(workload.Spec.Ports),
			Env:     env,
			EnvFrom: envFromSources(workload.Spec.EnvFrom),
		}}
		return nil
	})
	return err
}

func (r *WorkloadReconciler) reconcileService(ctx context.Context, workload *runtimeapi.Workload) error {
	service := &corev1.Service{}
	service.Name = workload.Name
	service.Namespace = workload.Namespace

	if len(workload.Spec.Ports) == 0 {
		if err := r.Get(ctx, client.ObjectKeyFromObject(workload), service); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		if !metav1.IsControlledBy(service, workload) {
			return nil
		}
		if err := r.Delete(ctx, service); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := controllerutil.SetControllerReference(workload, service, r.Scheme); err != nil {
			return err
		}

		service.Labels = workloadLabels(workload)
		service.Spec.Selector = workloadLabels(workload)
		service.Spec.Ports = servicePorts(workload.Spec.Ports)
		return nil
	})
	return err
}

func (r *WorkloadReconciler) reconcileStatus(ctx context.Context, workload *runtimeapi.Workload) error {
	var deployment appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKeyFromObject(workload), &deployment); err != nil {
		return err
	}

	nextStatus := workload.Status
	nextStatus.ObservedGeneration = workload.Generation
	nextStatus.ActiveImage = workload.Spec.Image
	nextStatus.Targets = []runtimeapi.WorkloadTargetStatus{workloadTargetStatus(workload, &deployment, r.clusterMemberRef())}

	apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Degraded")
	apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Pending")
	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "RuntimeObjectsApplied",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: workload.Generation,
		Reason:             "ApplySucceeded",
		Message:            "Deployment and Service desired state applied to the local cluster.",
	})
	apimeta.SetStatusCondition(&nextStatus.Conditions, availabilityCondition(workload, &deployment))

	if apiequality.Semantic.DeepEqual(workload.Status, nextStatus) {
		return nil
	}

	workload.Status = nextStatus
	return r.Status().Update(ctx, workload)
}

func workloadLabels(workload *runtimeapi.Workload) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "polykube-operator",
		workloadNameLabel:              workload.Name,
	}
}

func workloadTargetStatus(workload *runtimeapi.Workload, deployment *appsv1.Deployment, clusterMemberName string) runtimeapi.WorkloadTargetStatus {
	state := runtimeapi.WorkloadTargetStateReconciling
	message := "Deployment is reconciling in the local cluster."
	if deploymentAvailable(deployment) {
		state = runtimeapi.WorkloadTargetStateAvailable
		message = "Deployment is available in the local cluster."
	}

	now := metav1.Now()
	for _, target := range workload.Status.Targets {
		if target.ClusterMemberRef == clusterMemberName && target.State == state && target.LastTransitionTime != nil {
			now = *target.LastTransitionTime
			break
		}
	}

	return runtimeapi.WorkloadTargetStatus{
		ClusterMemberRef:   clusterMemberName,
		State:              state,
		RuntimeRef:         deployment.Name,
		LastTransitionTime: &now,
		Message:            message,
	}
}

func availabilityCondition(workload *runtimeapi.Workload, deployment *appsv1.Deployment) metav1.Condition {
	if deploymentAvailable(deployment) {
		return metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: workload.Generation,
			Reason:             "DeploymentAvailable",
			Message:            "Deployment is available in the local cluster.",
		}
	}

	return metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: workload.Generation,
		Reason:             "DeploymentReconciling",
		Message:            "Deployment has not reported availability yet.",
	}
}

func deploymentAvailable(deployment *appsv1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func metav1LabelSelector(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}

func imagePullSecrets(refs []runtimeapi.LocalObjectReference) []corev1.LocalObjectReference {
	secrets := make([]corev1.LocalObjectReference, 0, len(refs))
	for _, ref := range refs {
		secrets = append(secrets, corev1.LocalObjectReference{Name: ref.Name})
	}
	return secrets
}

func containerPorts(ports []runtimeapi.ContainerPort) []corev1.ContainerPort {
	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for _, port := range ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      protocol(port.Protocol),
		})
	}
	return containerPorts
}

func servicePorts(ports []runtimeapi.ContainerPort) []corev1.ServicePort {
	servicePorts := make([]corev1.ServicePort, 0, len(ports))
	for _, port := range ports {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       port.Name,
			Port:       port.ContainerPort,
			TargetPort: intstr.FromInt32(port.ContainerPort),
			Protocol:   protocol(port.Protocol),
		})
	}
	return servicePorts
}

func protocol(value string) corev1.Protocol {
	if value == "" {
		return corev1.ProtocolTCP
	}
	return corev1.Protocol(value)
}

func envVars(vars []runtimeapi.EnvVar) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(vars))
	for _, variable := range vars {
		env = append(env, corev1.EnvVar{Name: variable.Name, Value: variable.Value})
	}
	return env
}

func preserveDatastoreManagedEnvVars(desired, existing []corev1.EnvVar, managed map[string]bool) []corev1.EnvVar {
	if len(managed) == 0 {
		return desired
	}
	preserved := make([]corev1.EnvVar, 0, len(managed))
	for _, variable := range existing {
		if managed[variable.Name] {
			preserved = append(preserved, variable)
		}
	}
	return mergeEnvVars(desired, preserved)
}

func envFromSources(sources []runtimeapi.EnvFromSource) []corev1.EnvFromSource {
	envFrom := make([]corev1.EnvFromSource, 0, len(sources))
	for _, source := range sources {
		envFrom = append(envFrom, corev1.EnvFromSource{
			ConfigMapRef: configMapEnvSource(source.ConfigMapRef),
			SecretRef:    secretEnvSource(source.SecretRef),
		})
	}
	return envFrom
}

func configMapEnvSource(ref *runtimeapi.LocalObjectReference) *corev1.ConfigMapEnvSource {
	if ref == nil {
		return nil
	}
	return &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: ref.Name}}
}

func secretEnvSource(ref *runtimeapi.LocalObjectReference) *corev1.SecretEnvSource {
	if ref == nil {
		return nil
	}
	return &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: ref.Name}}
}
