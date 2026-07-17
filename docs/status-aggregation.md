# Multicluster Workload Status

Polykube provides a read-only CLI for viewing `Workload` status from multiple Kubernetes clusters in one result. It queries only kubeconfig contexts named by the caller and does not change the local-only reconciliation model.

## Query Status

From the repository root, enter the operator module and run the CLI with an explicit comma-separated context list:

```bash
cd operator
go run ./cmd/polykube-status --contexts development-us,development-eu
```

The default table includes:

- kubeconfig context
- namespace and Workload name
- cluster member reported by `status.targets[]`
- desired and observed generations
- target state
- active image
- target message

Query one namespace with `--namespace`:

```bash
go run ./cmd/polykube-status \
  --contexts development-us,development-eu \
  --namespace applications
```

Use JSON for automation:

```bash
go run ./cmd/polykube-status \
  --contexts development-us,development-eu \
  --output json
```

Use `--kubeconfig` to select one kubeconfig file explicitly. Without that flag, standard Kubernetes loading rules apply, including `KUBECONFIG` and `$HOME/.kube/config`.

The command returns no output and exits nonzero if any requested context cannot be loaded, reached, or authorized. This prevents a partial result from looking like complete federation state. A Workload that exists but has not reported a target yet remains visible with empty JSON status fields or `<none>` table values.

## Local Demo

After creating the local clusters and exporting the generated kubeconfig bundle as described in the [local multicluster demo](../examples/local-multicluster/README.md), run:

```bash
mise run local:workload:status
mise run local:workload:status -- --output json
```

The task passes `polykube-alpha,polykube-beta` explicitly by default. Override them with `--contexts`; the CLI never discovers and queries every available context implicitly.

## Security Boundary

The CLI runs on demand on the user's machine. It opens read requests to each selected Kubernetes API using that context's existing credentials, lists `Workload` resources, writes the combined result to standard output, and exits. It does not:

- run inside an operator or cluster
- write to any Kubernetes API
- copy status between clusters
- store kubeconfigs, credentials, or results
- give an operator credentials for another cluster
- introduce a continuously running central control plane

Credentials need only `list` access to `workloads.runtime.polykube.dev` in the queried namespaces. Operators continue to reconcile and write status only in their own cluster. Treat JSON output as operational metadata because status messages and image references may contain environment details.
