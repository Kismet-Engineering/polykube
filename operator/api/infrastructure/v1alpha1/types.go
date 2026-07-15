package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "infrastructure.polykube.dev"

type ClusterMemberSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Environment string `json:"environment,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName,omitempty"`
	// +kubebuilder:validation:Format=uri
	APIEndpoint string `json:"apiEndpoint,omitempty"`
	// +kubebuilder:validation:Format=cidr
	PodCIDR string `json:"podCIDR,omitempty"`
	// +kubebuilder:validation:Format=cidr
	ServiceCIDR string            `json:"serviceCIDR,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ClusterMemberStatus struct {
	ObservedGeneration int64        `json:"observedGeneration,omitempty"`
	LastObservedAt     *metav1.Time `json:"lastObservedAt,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pcm
type ClusterMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ClusterMemberSpec   `json:"spec,omitempty"`
	Status ClusterMemberStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClusterMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterMember `json:"items"`
}

// +kubebuilder:validation:Enum=ActivePassive;ActiveActive
type FederationRoutingMode string

const (
	FederationRoutingModeActivePassive FederationRoutingMode = "ActivePassive"
	FederationRoutingModeActiveActive  FederationRoutingMode = "ActiveActive"
)

type FederationMemberReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type FederationTargetPolicy struct {
	MemberSelector *metav1.LabelSelector `json:"memberSelector,omitempty"`
	// +kubebuilder:validation:items:MinLength=1
	Members []string `json:"members,omitempty"`
}

type FederationNetworkingSpec struct {
	// +kubebuilder:validation:MinLength=1
	Substrate string            `json:"substrate,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

type FederationSpec struct {
	MemberSelector      *metav1.LabelSelector       `json:"memberSelector,omitempty"`
	Members             []FederationMemberReference `json:"members,omitempty"`
	RoutingMode         FederationRoutingMode       `json:"routingMode,omitempty"`
	DefaultTargetPolicy *FederationTargetPolicy     `json:"defaultTargetPolicy,omitempty"`
	Networking          *FederationNetworkingSpec   `json:"networking,omitempty"`
}

type FederationMemberStatus struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Ready bool `json:"ready"`
}

type FederationStatus struct {
	ReadyMembers int32                    `json:"readyMembers,omitempty"`
	Members      []FederationMemberStatus `json:"members,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pfed
type Federation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   FederationSpec   `json:"spec,omitempty"`
	Status FederationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type FederationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Federation `json:"items"`
}
