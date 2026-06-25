package controller

import (
	"context"
	"fmt"
	"strings"

	dataapi "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const datastoreBindingFinalizer = "datastorebinding.data.polykube.dev/finalizer"

var acceptedEngines = []string{"yugabytedb", "postgres_compatible", "postgres"}

type DatastoreBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *DatastoreBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("datastorebinding", req.NamespacedName.String())

	var binding dataapi.DatastoreBinding
	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileBindingDelete(ctx, &binding)
	}

	if controllerutil.AddFinalizer(&binding, datastoreBindingFinalizer) {
		if err := r.Update(ctx, &binding); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Engine validation.
	if !acceptedEngine(binding.Spec.Engine) {
		return ctrl.Result{}, r.setDegradedStatus(ctx, &binding, "UnsupportedEngine",
			fmt.Sprintf("Engine %q is not supported. Accepted values: %s.", binding.Spec.Engine, strings.Join(acceptedEngines, ", ")))
	}

	// Resolve Workload.
	workload, err := r.resolveWorkload(ctx, &binding)
	if err != nil {
		return ctrl.Result{}, err
	}
	if workload == nil {
		return ctrl.Result{}, r.setDegradedStatus(ctx, &binding, "WorkloadNotFound",
			fmt.Sprintf("Workload %q not found.", binding.Spec.WorkloadRef.Name))
	}

	// Resolve connection secret.
	secret, err := r.resolveConnectionSecret(ctx, &binding)
	if err != nil {
		return ctrl.Result{}, err
	}
	if secret == nil {
		return ctrl.Result{}, r.setDegradedStatus(ctx, &binding, "ConnectionSecretNotFound",
			fmt.Sprintf("Secret %q not found. Ensure the secret exists before the DatastoreBinding is reconciled. See docs/architecture.md for the recommended secrets provisioning model.", connectionSecretName(&binding)))
	}

	// Inject env vars into the Deployment.
	if err := r.injectEnvVars(ctx, &binding, workload, secret); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.setReadyStatus(ctx, &binding); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("observed datastorebinding", "generation", binding.Generation)
	return ctrl.Result{}, nil
}

func (r *DatastoreBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataapi.DatastoreBinding{}).
		Complete(r)
}

func (r *DatastoreBindingReconciler) resolveWorkload(ctx context.Context, binding *dataapi.DatastoreBinding) (*runtimeapi.Workload, error) {
	ns := binding.Spec.WorkloadRef.Namespace
	if ns == "" {
		ns = binding.Namespace
	}
	var workload runtimeapi.Workload
	if err := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: binding.Spec.WorkloadRef.Name}, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &workload, nil
}

func (r *DatastoreBindingReconciler) resolveConnectionSecret(ctx context.Context, binding *dataapi.DatastoreBinding) (*corev1.Secret, error) {
	ns := binding.Spec.ConnectionRef.Namespace
	if ns == "" {
		ns = binding.Namespace
	}
	var secret corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: binding.Spec.ConnectionRef.Name}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &secret, nil
}

func (r *DatastoreBindingReconciler) injectEnvVars(ctx context.Context, binding *dataapi.DatastoreBinding, workload *runtimeapi.Workload, secret *corev1.Secret) error {
	var deployment appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: workload.Name}, &deployment); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		nameUpper := strings.ToUpper(strings.ReplaceAll(binding.Name, "-", "_"))
		secretKey := connectionSecretKey(secret)

		envVarsToInject := []corev1.EnvVar{
			{
				Name: "DATASTORE_" + nameUpper + "_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
						Key:                  secretKey,
					},
				},
			},
			{
				Name:  "DATASTORE_" + nameUpper + "_REPLICATION_MODE",
				Value: string(binding.Spec.ReplicationMode),
			},
		}
		if binding.Name == "primary" {
			envVarsToInject = append(envVarsToInject, corev1.EnvVar{
				Name: "DATABASE_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
						Key:                  secretKey,
					},
				},
			})
		}

		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name == "app" {
				deployment.Spec.Template.Spec.Containers[i].Env = mergeEnvVars(
					deployment.Spec.Template.Spec.Containers[i].Env,
					envVarsToInject,
				)
				break
			}
		}
		return nil
	})
	return err
}

func (r *DatastoreBindingReconciler) reconcileBindingDelete(ctx context.Context, binding *dataapi.DatastoreBinding) error {
	if !controllerutil.ContainsFinalizer(binding, datastoreBindingFinalizer) {
		return nil
	}

	workload, err := r.resolveWorkload(ctx, binding)
	if err != nil {
		return err
	}
	if workload != nil {
		var deployment appsv1.Deployment
		if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: workload.Name}, &deployment); err == nil {
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
				nameUpper := strings.ToUpper(strings.ReplaceAll(binding.Name, "-", "_"))
				keysToRemove := map[string]bool{
					"DATASTORE_" + nameUpper + "_URL":              true,
					"DATASTORE_" + nameUpper + "_REPLICATION_MODE": true,
				}
				if binding.Name == "primary" {
					keysToRemove["DATABASE_URL"] = true
				}
				for i := range deployment.Spec.Template.Spec.Containers {
					if deployment.Spec.Template.Spec.Containers[i].Name == "app" {
						deployment.Spec.Template.Spec.Containers[i].Env = removeEnvVars(
							deployment.Spec.Template.Spec.Containers[i].Env,
							keysToRemove,
						)
						break
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if !apierrors.IsNotFound(err) {
			return err
		}
	}

	controllerutil.RemoveFinalizer(binding, datastoreBindingFinalizer)
	return r.Update(ctx, binding)
}

func (r *DatastoreBindingReconciler) setDegradedStatus(ctx context.Context, binding *dataapi.DatastoreBinding, reason, message string) error {
	nextStatus := binding.Status
	nextStatus.ObservedGeneration = binding.Generation
	nextStatus.Message = message

	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: binding.Generation,
		Reason:             reason,
		Message:            message,
	})
	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: binding.Generation,
		Reason:             reason,
		Message:            message,
	})

	if apiequality.Semantic.DeepEqual(binding.Status, nextStatus) {
		return nil
	}
	binding.Status = nextStatus
	return r.Status().Update(ctx, binding)
}

func (r *DatastoreBindingReconciler) setReadyStatus(ctx context.Context, binding *dataapi.DatastoreBinding) error {
	nextStatus := binding.Status
	nextStatus.ObservedGeneration = binding.Generation
	nextStatus.Message = "DatastoreBinding is ready and env vars have been injected into the Deployment."

	apimeta.RemoveStatusCondition(&nextStatus.Conditions, "Degraded")
	apimeta.SetStatusCondition(&nextStatus.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: binding.Generation,
		Reason:             "Reconciled",
		Message:            "Connection secret resolved and env vars injected.",
	})

	if apiequality.Semantic.DeepEqual(binding.Status, nextStatus) {
		return nil
	}
	binding.Status = nextStatus
	return r.Status().Update(ctx, binding)
}

func acceptedEngine(engine string) bool {
	for _, e := range acceptedEngines {
		if e == engine {
			return true
		}
	}
	return false
}

func connectionSecretName(binding *dataapi.DatastoreBinding) string {
	return binding.Spec.ConnectionRef.Name
}

// connectionSecretKey returns the key in the secret that holds the connection URL.
// Prefers "url", falls back to "DATABASE_URL".
func connectionSecretKey(secret *corev1.Secret) string {
	if _, ok := secret.Data["url"]; ok {
		return "url"
	}
	return "DATABASE_URL"
}

// mergeEnvVars merges toInject into existing, replacing by name where present.
func mergeEnvVars(existing, toInject []corev1.EnvVar) []corev1.EnvVar {
	index := map[string]int{}
	for i, e := range existing {
		index[e.Name] = i
	}
	result := append([]corev1.EnvVar{}, existing...)
	for _, v := range toInject {
		if i, ok := index[v.Name]; ok {
			result[i] = v
		} else {
			result = append(result, v)
		}
	}
	return result
}

// removeEnvVars removes env vars whose names are in the keys map.
func removeEnvVars(existing []corev1.EnvVar, keys map[string]bool) []corev1.EnvVar {
	result := existing[:0]
	for _, e := range existing {
		if !keys[e.Name] {
			result = append(result, e)
		}
	}
	return result
}
