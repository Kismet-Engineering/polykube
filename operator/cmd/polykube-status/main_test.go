package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRunTableAggregatesExplicitContexts(t *testing.T) {
	lists := map[string]*unstructured.UnstructuredList{
		"polykube-alpha": workloadList("default", "echo", 3, 3, "example/echo:v2", "alpha", "Available", "rollout complete"),
		"polykube-beta":  workloadList("default", "echo", 3, 2, "example/echo:v1", "beta", "Reconciling", "waiting for deployment"),
	}
	list := func(_ context.Context, _, contextName, namespace string) (*unstructured.UnstructuredList, error) {
		if namespace != "default" {
			t.Fatalf("namespace = %q, want default", namespace)
		}
		return lists[contextName], nil
	}

	var stdout strings.Builder
	err := run(context.Background(), []string{
		"--contexts", "polykube-beta,polykube-alpha",
		"--namespace", "default",
	}, &stdout, &strings.Builder{}, list)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"CONTEXT",
		"polykube-alpha  default    echo      alpha",
		"3           3         Available",
		"example/echo:v2  rollout complete",
		"polykube-beta   default    echo      beta",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output does not contain %q:\n%s", expected, output)
		}
	}
	if strings.Index(output, "polykube-alpha") > strings.Index(output, "polykube-beta") {
		t.Fatalf("output is not sorted by context:\n%s", output)
	}
}

func TestRunJSONIncludesWorkloadWithoutTargetStatus(t *testing.T) {
	list := func(_ context.Context, _, _, _ string) (*unstructured.UnstructuredList, error) {
		return workloadList("apps", "api", 4, 0, "", "", "", ""), nil
	}

	var stdout strings.Builder
	if err := run(context.Background(), []string{"--contexts", "development", "--output", "json"}, &stdout, &strings.Builder{}, list); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	var records []statusRecord
	if err := json.Unmarshal([]byte(stdout.String()), &records); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one", records)
	}
	if records[0].Context != "development" || records[0].Generation != 4 || records[0].TargetState != "" {
		t.Fatalf("record = %#v", records[0])
	}
}

func TestRunJSONUsesEmptyArrayWhenNoWorkloadsExist(t *testing.T) {
	list := func(_ context.Context, _, _, _ string) (*unstructured.UnstructuredList, error) {
		return &unstructured.UnstructuredList{}, nil
	}

	var stdout strings.Builder
	if err := run(context.Background(), []string{"--contexts", "development", "--output", "json"}, &stdout, &strings.Builder{}, list); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("output = %q, want empty JSON array", stdout.String())
	}
}

func TestRunRejectsImplicitOrPartialResults(t *testing.T) {
	t.Run("contexts required", func(t *testing.T) {
		err := run(context.Background(), nil, &strings.Builder{}, &strings.Builder{}, nil)
		if err == nil || !strings.Contains(err.Error(), "--contexts is required") {
			t.Fatalf("run() error = %v", err)
		}
	})

	t.Run("context failure", func(t *testing.T) {
		list := func(_ context.Context, _, contextName, _ string) (*unstructured.UnstructuredList, error) {
			if contextName == "beta" {
				return nil, errors.New("access denied")
			}
			return workloadList("default", "echo", 1, 1, "image", "alpha", "Available", ""), nil
		}
		var stdout strings.Builder
		err := run(context.Background(), []string{"--contexts", "alpha,beta"}, &stdout, &strings.Builder{}, list)
		if err == nil || !strings.Contains(err.Error(), `query context "beta": access denied`) {
			t.Fatalf("run() error = %v", err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("partial output = %q, want none", stdout.String())
		}
	})
}

func TestRunReturnsTableWriteFailure(t *testing.T) {
	list := func(_ context.Context, _, _, _ string) (*unstructured.UnstructuredList, error) {
		return workloadList("default", "echo", 1, 1, "image", "alpha", "Available", ""), nil
	}
	err := run(context.Background(), []string{"--contexts", "alpha"}, failingWriter{}, &strings.Builder{}, list)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("run() error = %v, want closed pipe", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func workloadList(namespace, name string, generation, observedGeneration int64, image, member, state, message string) *unstructured.UnstructuredList {
	status := map[string]any{
		"observedGeneration": observedGeneration,
		"activeImage":        image,
	}
	if member != "" {
		status["targets"] = []any{map[string]any{
			"clusterMemberRef": member,
			"state":            state,
			"message":          message,
		}}
	}
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": "runtime.polykube.dev/v1alpha1",
		"kind":       "Workload",
		"metadata": map[string]any{
			"namespace":  namespace,
			"name":       name,
			"generation": generation,
		},
		"status": status,
	}}}}
}
