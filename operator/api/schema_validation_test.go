package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestGeneratedCRDSchemaConstraints(t *testing.T) {
	tests := []struct {
		name       string
		crd        string
		required   [][]string
		nonEmpty   [][]string
		enums      map[string][]string
		formats    map[string]string
		validation map[string]string
	}{
		{
			name: "ClusterMember",
			crd:  "infrastructure.polykube.dev_clustermembers.yaml",
			required: [][]string{
				{"spec", "provider"}, {"spec", "region"}, {"spec", "clusterName"},
			},
			nonEmpty: [][]string{
				{"spec", "provider"}, {"spec", "region"}, {"spec", "clusterName"},
			},
			formats: map[string]string{
				"spec.apiEndpoint": "uri", "spec.podCIDR": "cidr", "spec.serviceCIDR": "cidr",
			},
		},
		{
			name:     "Federation",
			crd:      "infrastructure.polykube.dev_federations.yaml",
			nonEmpty: [][]string{{"spec", "members", "[]", "name"}, {"spec", "defaultTargetPolicy", "members", "[]"}},
			enums: map[string][]string{
				"spec.routingMode": {"ActivePassive", "ActiveActive"},
			},
		},
		{
			name: "Workload",
			crd:  "runtime.polykube.dev_workloads.yaml",
			required: [][]string{
				{"spec", "federationRef"}, {"spec", "image"}, {"spec", "env", "[]", "name"},
			},
			nonEmpty: [][]string{
				{"spec", "image"}, {"spec", "env", "[]", "name"},
				{"spec", "imagePullSecrets", "[]", "name"}, {"spec", "targetPolicy", "members", "[]"},
			},
			enums: map[string][]string{
				"spec.ports.[].protocol": {"TCP", "UDP", "SCTP"},
			},
			validation: map[string]string{
				"spec.envFrom.[]": "has(self.configMapRef) || has(self.secretRef)",
			},
		},
		{
			name:     "ServiceEndpoint",
			crd:      "routing.polykube.dev_serviceendpoints.yaml",
			required: [][]string{{"spec", "workloadRef"}, {"spec", "routingMode"}},
			nonEmpty: [][]string{{"spec", "workloadRef", "name"}, {"spec", "hostnames", "[]"}, {"spec", "primaryMemberRef"}},
			enums: map[string][]string{
				"spec.routingMode": {"ActivePassive", "ActiveActive"},
			},
			validation: map[string]string{
				"spec": "self.routingMode != 'ActivePassive' || has(self.primaryMemberRef)",
			},
		},
		{
			name: "DatastoreBinding",
			crd:  "data.polykube.dev_datastorebindings.yaml",
			required: [][]string{
				{"spec", "workloadRef"}, {"spec", "engine"}, {"spec", "connectionRef"}, {"spec", "replicationMode"},
			},
			nonEmpty: [][]string{{"spec", "workloadRef", "name"}, {"spec", "connectionRef", "name"}},
			enums: map[string][]string{
				"spec.engine":          {"yugabytedb", "postgres_compatible", "postgres"},
				"spec.replicationMode": {"None", "ActivePassive", "ActiveActive"},
				"spec.conflictPolicy":  {"Reject", "LastWriteWins", "External"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := loadSchema(t, tt.crd)
			for _, path := range tt.required {
				parent := schemaAt(t, schema, path[:len(path)-1]...)
				if !slices.Contains(stringValues(t, parent["required"]), path[len(path)-1]) {
					t.Errorf("%s is not required", pathString(path))
				}
			}
			for _, path := range tt.nonEmpty {
				field := schemaAt(t, schema, path...)
				if field["minLength"] != float64(1) {
					t.Errorf("%s does not require a non-empty string", pathString(path))
				}
			}
			for path, want := range tt.enums {
				field := schemaAt(t, schema, strings.Split(path, ".")...)
				if got := enumValues(t, field); !slices.Equal(got, want) {
					t.Errorf("%s enum = %v, want %v", path, got, want)
				}
			}
			for path, want := range tt.formats {
				if got := schemaAt(t, schema, strings.Split(path, ".")...)["format"]; got != want {
					t.Errorf("%s format = %q, want %q", path, got, want)
				}
			}
			for path, want := range tt.validation {
				field := schemaAt(t, schema, strings.Split(path, ".")...)
				validations, ok := field["x-kubernetes-validations"].([]any)
				if !ok || len(validations) != 1 || validations[0].(map[string]any)["rule"] != want {
					t.Errorf("%s validation = %v, want rule %q", path, validations, want)
				}
			}
		})
	}
}

func loadSchema(t *testing.T, name string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "config", "crd", "bases", name))
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := yaml.ToJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var crd map[string]any
	if err := json.Unmarshal(jsonData, &crd); err != nil {
		t.Fatal(err)
	}
	versions := crd["spec"].(map[string]any)["versions"].([]any)
	for _, rawVersion := range versions {
		version := rawVersion.(map[string]any)
		if version["name"] == "v1alpha1" {
			return version["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)
		}
	}
	t.Fatalf("CRD %s has no v1alpha1 schema", name)
	return nil
}

func schemaAt(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()
	current := schema
	for _, segment := range path {
		if segment == "[]" {
			next, ok := current["items"].(map[string]any)
			if !ok {
				t.Fatalf("%s has no item schema", pathString(path))
			}
			current = next
			continue
		}
		properties, ok := current["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no properties", pathString(path))
		}
		next, ok := properties[segment].(map[string]any)
		if !ok {
			t.Fatalf("%s has no schema", pathString(path))
		}
		current = next
	}
	return current
}

func enumValues(t *testing.T, schema map[string]any) []string {
	t.Helper()
	return stringValues(t, schema["enum"])
}

func stringValues(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	values := make([]string, len(raw))
	for i := range raw {
		var ok bool
		if values[i], ok = raw[i].(string); !ok {
			t.Fatalf("value %v is not a string", raw[i])
		}
	}
	return values
}

func pathString(path []string) string {
	if len(path) == 0 {
		return "<root>"
	}
	return strings.Join(path, ".")
}
