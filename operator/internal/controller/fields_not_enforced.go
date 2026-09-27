package controller

import (
	"fmt"
	"strings"

	dataapi "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	routingapi "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fieldsNotEnforcedCondition is informational: it reports user-set spec fields
// that are accepted for future integration but not acted on by this operator.
// It never affects Ready, Degraded, or Available.
const fieldsNotEnforcedCondition = "FieldsNotEnforced"

func setFieldsNotEnforcedCondition(conditions *[]metav1.Condition, generation int64, fields []string) {
	if len(fields) == 0 {
		apimeta.RemoveStatusCondition(conditions, fieldsNotEnforcedCondition)
		return
	}
	verb := "is"
	if len(fields) > 1 {
		verb = "are"
	}
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               fieldsNotEnforcedCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             "AcceptedNotEnforced",
		Message:            fmt.Sprintf("%s %s accepted but not acted on by the v1alpha1 operator. See docs/api-field-support.md.", strings.Join(fields, ", "), verb),
	})
}

func workloadFieldsNotEnforced(spec *runtimeapi.WorkloadSpec) []string {
	var fields []string
	if spec.TargetPolicy != nil && spec.TargetPolicy.Strategy != "" {
		fields = append(fields, "spec.targetPolicy.strategy")
	}
	if spec.RolloutRef != nil {
		fields = append(fields, "spec.rolloutRef")
	}
	return fields
}

func serviceEndpointFieldsNotEnforced(spec *routingapi.ServiceEndpointSpec) []string {
	var fields []string
	if spec.FailoverPolicy != nil {
		fields = append(fields, "spec.failoverPolicy")
	}
	if spec.GatewayRef != nil {
		fields = append(fields, "spec.gatewayRef")
	}
	return fields
}

func datastoreBindingFieldsNotEnforced(spec *dataapi.DatastoreBindingSpec) []string {
	if spec.ConflictPolicy != "" {
		return []string{"spec.conflictPolicy"}
	}
	return nil
}

func federationFieldsNotEnforced(spec *infrastructure.FederationSpec) []string {
	if spec.DefaultTargetPolicy != nil {
		return []string{"spec.defaultTargetPolicy"}
	}
	return nil
}
