package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "routing.polykube.dev"

type NamespacedObjectReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// +kubebuilder:validation:Enum=ActivePassive;ActiveActive
type RoutingMode string

const (
	RoutingModeActivePassive RoutingMode = "ActivePassive"
	RoutingModeActiveActive  RoutingMode = "ActiveActive"
)

type GatewayReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Section string `json:"section,omitempty"`
}

type FailoverPolicy struct {
	Enabled         bool   `json:"enabled,omitempty"`
	HealthThreshold string `json:"healthThreshold,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.routingMode != 'ActivePassive' || has(self.primaryMemberRef)",message="primaryMemberRef is required for ActivePassive routing"
type ServiceEndpointSpec struct {
	WorkloadRef NamespacedObjectReference `json:"workloadRef"`
	// +kubebuilder:validation:items:MinLength=1
	Hostnames []string `json:"hostnames,omitempty"`
	// +kubebuilder:validation:Required
	RoutingMode RoutingMode `json:"routingMode,omitempty"`
	// +kubebuilder:validation:MinLength=1
	PrimaryMemberRef string            `json:"primaryMemberRef,omitempty"`
	FailoverPolicy   *FailoverPolicy   `json:"failoverPolicy,omitempty"`
	GatewayRef       *GatewayReference `json:"gatewayRef,omitempty"`
}

type ServiceEndpointStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ActiveMemberRef    string             `json:"activeMemberRef,omitempty"`
	ResolvedHostnames  []string           `json:"resolvedHostnames,omitempty"`
	LastTransitionTime *metav1.Time       `json:"lastTransitionTime,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=pse
type ServiceEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ServiceEndpointSpec   `json:"spec,omitempty"`
	Status ServiceEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ServiceEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceEndpoint `json:"items"`
}
