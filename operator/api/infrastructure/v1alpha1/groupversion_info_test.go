package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersInfrastructureKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	tests := []struct {
		name string
		obj  runtime.Object
		kind string
	}{
		{name: "cluster member", obj: &ClusterMember{}, kind: "ClusterMember"},
		{name: "federation", obj: &Federation{}, kind: "Federation"},
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
			if got := gvks[0]; got.Group != GroupName || got.Version != "v1alpha1" || got.Kind != tt.kind {
				t.Fatalf("GVK = %s, want %s/v1alpha1, Kind=%s", got.String(), GroupName, tt.kind)
			}
		})
	}
}
