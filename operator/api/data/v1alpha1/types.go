package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const GroupName = "data.polykube.dev"

type NamespacedObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type DatastoreReplicationMode string

const (
	DatastoreReplicationModeNone          DatastoreReplicationMode = "None"
	DatastoreReplicationModeActivePassive DatastoreReplicationMode = "ActivePassive"
	DatastoreReplicationModeActiveActive  DatastoreReplicationMode = "ActiveActive"
)

type DatastoreConflictPolicy string

const (
	DatastoreConflictPolicyReject        DatastoreConflictPolicy = "Reject"
	DatastoreConflictPolicyLastWriteWins DatastoreConflictPolicy = "LastWriteWins"
	DatastoreConflictPolicyExternal      DatastoreConflictPolicy = "External"
)

type DatastoreBindingSpec struct {
	WorkloadRef     NamespacedObjectReference `json:"workloadRef"`
	Engine          string                    `json:"engine"`
	ConnectionRef   NamespacedObjectReference `json:"connectionRef"`
	ReplicationMode DatastoreReplicationMode  `json:"replicationMode,omitempty"`
	ConflictPolicy  DatastoreConflictPolicy   `json:"conflictPolicy,omitempty"`
}

type DatastoreBindingStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Message            string             `json:"message,omitempty"`
}

type DatastoreBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatastoreBindingSpec   `json:"spec,omitempty"`
	Status DatastoreBindingStatus `json:"status,omitempty"`
}

type DatastoreBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DatastoreBinding `json:"items"`
}
