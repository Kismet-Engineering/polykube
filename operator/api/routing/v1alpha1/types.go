package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "routing.polykube.dev"

type NamespacedObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type RoutingMode string

const (
	RoutingModeActivePassive RoutingMode = "ActivePassive"
	RoutingModeActiveActive  RoutingMode = "ActiveActive"
)

type GatewayReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Section   string `json:"section,omitempty"`
}

type FailoverPolicy struct {
	Enabled         bool   `json:"enabled,omitempty"`
	HealthThreshold string `json:"healthThreshold,omitempty"`
}

type ServiceEndpointSpec struct {
	WorkloadRef      NamespacedObjectReference `json:"workloadRef"`
	Hostnames        []string                  `json:"hostnames,omitempty"`
	RoutingMode      RoutingMode               `json:"routingMode,omitempty"`
	PrimaryMemberRef string                    `json:"primaryMemberRef,omitempty"`
	FailoverPolicy   *FailoverPolicy           `json:"failoverPolicy,omitempty"`
	GatewayRef       *GatewayReference         `json:"gatewayRef,omitempty"`
}

type ServiceEndpointStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ActiveMemberRef    string             `json:"activeMemberRef,omitempty"`
	ResolvedHostnames  []string           `json:"resolvedHostnames,omitempty"`
	LastTransitionTime *metav1.Time       `json:"lastTransitionTime,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

type ServiceEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceEndpointSpec   `json:"spec,omitempty"`
	Status ServiceEndpointStatus `json:"status,omitempty"`
}

type ServiceEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceEndpoint `json:"items"`
}
