package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "data.polykube.dev"

type NamespacedObjectReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// +kubebuilder:validation:Enum=None;ActivePassive;ActiveActive
type DatastoreReplicationMode string

const (
	DatastoreReplicationModeNone          DatastoreReplicationMode = "None"
	DatastoreReplicationModeActivePassive DatastoreReplicationMode = "ActivePassive"
	DatastoreReplicationModeActiveActive  DatastoreReplicationMode = "ActiveActive"
)

// +kubebuilder:validation:Enum=Reject;LastWriteWins;External
type DatastoreConflictPolicy string

const (
	DatastoreConflictPolicyReject        DatastoreConflictPolicy = "Reject"
	DatastoreConflictPolicyLastWriteWins DatastoreConflictPolicy = "LastWriteWins"
	DatastoreConflictPolicyExternal      DatastoreConflictPolicy = "External"
)

type DatastoreBindingSpec struct {
	WorkloadRef NamespacedObjectReference `json:"workloadRef"`
	// +kubebuilder:validation:Enum=yugabytedb;postgres_compatible;postgres
	Engine        string                    `json:"engine"`
	ConnectionRef NamespacedObjectReference `json:"connectionRef"`
	// +kubebuilder:validation:Required
	ReplicationMode DatastoreReplicationMode `json:"replicationMode,omitempty"`
	ConflictPolicy  DatastoreConflictPolicy  `json:"conflictPolicy,omitempty"`
}

type DatastoreBindingStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Message            string             `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=pdb
type DatastoreBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   DatastoreBindingSpec   `json:"spec,omitempty"`
	Status DatastoreBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type DatastoreBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DatastoreBinding `json:"items"`
}
