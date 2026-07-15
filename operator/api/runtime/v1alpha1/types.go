package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "runtime.polykube.dev"

type LocalObjectReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type NamespacedObjectReference struct {
	// +kubebuilder:validation:Required
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type EnvFromSource struct {
	ConfigMapRef *LocalObjectReference `json:"configMapRef,omitempty"`
	SecretRef    *LocalObjectReference `json:"secretRef,omitempty"`
}

type ContainerPort struct {
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

type WorkloadTargetPolicy struct {
	MemberSelector *metav1.LabelSelector `json:"memberSelector,omitempty"`
	Members        []string              `json:"members,omitempty"`
	Strategy       string                `json:"strategy,omitempty"`
}

type RolloutReference struct {
	APIGroup string `json:"apiGroup,omitempty"`
	Kind     string `json:"kind,omitempty"`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type WorkloadSpec struct {
	FederationRef    NamespacedObjectReference `json:"federationRef"`
	Image            string                    `json:"image"`
	ImagePullSecrets []LocalObjectReference    `json:"imagePullSecrets,omitempty"`
	Ports            []ContainerPort           `json:"ports,omitempty"`
	Env              []EnvVar                  `json:"env,omitempty"`
	EnvFrom          []EnvFromSource           `json:"envFrom,omitempty"`
	TargetPolicy     *WorkloadTargetPolicy     `json:"targetPolicy,omitempty"`
	RolloutRef       *RolloutReference         `json:"rolloutRef,omitempty"`
	// +kubebuilder:validation:Minimum=0
	Replicas           *int32 `json:"replicas,omitempty"`
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// +kubebuilder:validation:Enum=Pending;Reconciling;Available;Degraded;Failed
type WorkloadTargetState string

const (
	WorkloadTargetStatePending     WorkloadTargetState = "Pending"
	WorkloadTargetStateReconciling WorkloadTargetState = "Reconciling"
	WorkloadTargetStateAvailable   WorkloadTargetState = "Available"
	WorkloadTargetStateDegraded    WorkloadTargetState = "Degraded"
	WorkloadTargetStateFailed      WorkloadTargetState = "Failed"
)

type WorkloadTargetStatus struct {
	// +kubebuilder:validation:Required
	ClusterMemberRef string `json:"clusterMemberRef"`
	// +kubebuilder:validation:Required
	State              WorkloadTargetState `json:"state"`
	RuntimeRef         string              `json:"runtimeRef,omitempty"`
	LastTransitionTime *metav1.Time        `json:"lastTransitionTime,omitempty"`
	Message            string              `json:"message,omitempty"`
}

type WorkloadStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions         []metav1.Condition     `json:"conditions,omitempty"`
	ObservedGeneration int64                  `json:"observedGeneration,omitempty"`
	Targets            []WorkloadTargetStatus `json:"targets,omitempty"`
	ActiveImage        string                 `json:"activeImage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=pwl
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   WorkloadSpec   `json:"spec,omitempty"`
	Status WorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}
