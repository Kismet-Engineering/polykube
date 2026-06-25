package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersRoutingKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	gvks, _, err := scheme.ObjectKinds(&ServiceEndpoint{})
	if err != nil {
		t.Fatalf("ObjectKinds() error = %v", err)
	}
	if len(gvks) != 1 {
		t.Fatalf("ObjectKinds() returned %d GVKs, want 1", len(gvks))
	}
	if got := gvks[0]; got.Group != GroupName || got.Version != "v1alpha1" || got.Kind != "ServiceEndpoint" {
		t.Fatalf("GVK = %s, want %s/v1alpha1, Kind=ServiceEndpoint", got.String(), GroupName)
	}
}
