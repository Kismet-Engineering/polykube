package scheme

import (
	"testing"

	data "github.com/Kismet-Engineering/polykube/operator/api/data/v1alpha1"
	infrastructure "github.com/Kismet-Engineering/polykube/operator/api/infrastructure/v1alpha1"
	routing "github.com/Kismet-Engineering/polykube/operator/api/routing/v1alpha1"
	runtimeapi "github.com/Kismet-Engineering/polykube/operator/api/runtime/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewRegistersPolykubeKinds(t *testing.T) {
	scheme, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name  string
		obj   runtime.Object
		group string
		kind  string
	}{
		{name: "cluster member", obj: &infrastructure.ClusterMember{}, group: infrastructure.GroupName, kind: "ClusterMember"},
		{name: "federation", obj: &infrastructure.Federation{}, group: infrastructure.GroupName, kind: "Federation"},
		{name: "workload", obj: &runtimeapi.Workload{}, group: runtimeapi.GroupName, kind: "Workload"},
		{name: "service endpoint", obj: &routing.ServiceEndpoint{}, group: routing.GroupName, kind: "ServiceEndpoint"},
		{name: "datastore binding", obj: &data.DatastoreBinding{}, group: data.GroupName, kind: "DatastoreBinding"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvks, _, err := scheme.ObjectKinds(tt.obj)
			if err != nil {
				t.Fatalf("ObjectKinds() error = %v", err)
			}
			if len(gvks) != 1 {
				t.Fatalf("ObjectKinds() returned %d GVKs, want 1", len(gvks))
			}
			if got := gvks[0]; got.Group != tt.group || got.Version != "v1alpha1" || got.Kind != tt.kind {
				t.Fatalf("GVK = %s, want %s/v1alpha1, Kind=%s", got.String(), tt.group, tt.kind)
			}
		})
	}
}
