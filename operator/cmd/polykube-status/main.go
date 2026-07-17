package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var workloadResource = schema.GroupVersionResource{
	Group: "runtime.polykube.dev", Version: "v1alpha1", Resource: "workloads",
}

type options struct {
	contexts   []string
	kubeconfig string
	namespace  string
	output     string
}

type statusRecord struct {
	Context            string `json:"context"`
	Namespace          string `json:"namespace"`
	Workload           string `json:"workload"`
	ClusterMember      string `json:"clusterMember"`
	Generation         int64  `json:"generation"`
	ObservedGeneration int64  `json:"observedGeneration"`
	TargetState        string `json:"targetState"`
	ActiveImage        string `json:"activeImage"`
	Message            string `json:"message"`
}

type listWorkloadsFunc func(context.Context, string, string, string) (*unstructured.UnstructuredList, error)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, listWorkloads); err != nil {
		fmt.Fprintf(os.Stderr, "polykube-status: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, list listWorkloadsFunc) error {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}

	records := make([]statusRecord, 0)
	for _, contextName := range opts.contexts {
		workloads, err := list(ctx, opts.kubeconfig, contextName, opts.namespace)
		if err != nil {
			return fmt.Errorf("query context %q: %w", contextName, err)
		}
		records = append(records, recordsFor(contextName, workloads)...)
	}

	sort.Slice(records, func(i, j int) bool {
		left := records[i]
		right := records[j]
		return strings.Join([]string{left.Context, left.Namespace, left.Workload, left.ClusterMember}, "\x00") <
			strings.Join([]string{right.Context, right.Namespace, right.Workload, right.ClusterMember}, "\x00")
	})

	switch opts.output {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(records)
	case "table":
		return writeTable(stdout, records)
	default:
		return fmt.Errorf("unsupported output %q", opts.output)
	}
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	flags := flag.NewFlagSet("polykube-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	contextsValue := flags.String("contexts", "", "Comma-separated kubeconfig contexts to query (required).")
	kubeconfig := flags.String("kubeconfig", "", "Path to a kubeconfig file. Defaults to standard kubeconfig loading rules.")
	namespace := flags.String("namespace", "", "Namespace to query. Defaults to all namespaces.")
	output := flags.String("output", "table", "Output format: table or json.")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	var contexts []string
	seen := make(map[string]struct{})
	for _, value := range strings.Split(*contextsValue, ",") {
		contextName := strings.TrimSpace(value)
		if contextName == "" {
			continue
		}
		if _, exists := seen[contextName]; exists {
			return options{}, fmt.Errorf("context %q was specified more than once", contextName)
		}
		seen[contextName] = struct{}{}
		contexts = append(contexts, contextName)
	}
	if len(contexts) == 0 {
		return options{}, errors.New("--contexts is required; refusing to query implicit kubeconfig contexts")
	}
	if *output != "table" && *output != "json" {
		return options{}, fmt.Errorf("--output must be table or json, got %q", *output)
	}

	return options{
		contexts:   contexts,
		kubeconfig: *kubeconfig,
		namespace:  *namespace,
		output:     *output,
	}, nil
}

func listWorkloads(ctx context.Context, kubeconfig, contextName, namespace string) (*unstructured.UnstructuredList, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client.Resource(workloadResource).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

func recordsFor(contextName string, workloads *unstructured.UnstructuredList) []statusRecord {
	var records []statusRecord
	for _, workload := range workloads.Items {
		observedGeneration, _, _ := unstructured.NestedInt64(workload.Object, "status", "observedGeneration")
		activeImage, _, _ := unstructured.NestedString(workload.Object, "status", "activeImage")
		targets, _, _ := unstructured.NestedSlice(workload.Object, "status", "targets")
		if len(targets) == 0 {
			records = append(records, statusRecord{
				Context:            contextName,
				Namespace:          workload.GetNamespace(),
				Workload:           workload.GetName(),
				Generation:         workload.GetGeneration(),
				ObservedGeneration: observedGeneration,
				ActiveImage:        activeImage,
			})
			continue
		}

		for _, target := range targets {
			targetMap, ok := target.(map[string]any)
			if !ok {
				continue
			}
			records = append(records, statusRecord{
				Context:            contextName,
				Namespace:          workload.GetNamespace(),
				Workload:           workload.GetName(),
				ClusterMember:      stringField(targetMap, "clusterMemberRef"),
				Generation:         workload.GetGeneration(),
				ObservedGeneration: observedGeneration,
				TargetState:        stringField(targetMap, "state"),
				ActiveImage:        activeImage,
				Message:            stringField(targetMap, "message"),
			})
		}
	}
	return records
}

func stringField(value map[string]any, field string) string {
	result, _ := value[field].(string)
	return result
}

func writeTable(output io.Writer, records []statusRecord) error {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "CONTEXT\tNAMESPACE\tWORKLOAD\tMEMBER\tGENERATION\tOBSERVED\tSTATE\tACTIVE IMAGE\tMESSAGE"); err != nil {
		return err
	}
	for _, record := range records {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			record.Context,
			record.Namespace,
			record.Workload,
			valueOrNone(record.ClusterMember),
			record.Generation,
			record.ObservedGeneration,
			valueOrNone(record.TargetState),
			valueOrNone(record.ActiveImage),
			valueOrNone(record.Message),
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func valueOrNone(value string) string {
	if value == "" {
		return "<none>"
	}
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(value)
}
