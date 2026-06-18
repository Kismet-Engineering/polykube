package controller

import (
	"context"

	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Scheme *runtime.Scheme
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

	if err := r.reconcileDeployment(ctx, &workload); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, &workload); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed workload", "generation", workload.Generation)
	return ctrl.Result{}, nil
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
		deployment.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:    "app",
			Image:   workload.Spec.Image,
			Ports:   containerPorts(workload.Spec.Ports),
			Env:     envVars(workload.Spec.Env),
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

func workloadLabels(workload *runtimeapi.Workload) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "polykube-operator",
		workloadNameLabel:              workload.Name,
	}
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
