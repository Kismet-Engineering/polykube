# Operator Security Model

Polykube is experimental public alpha software. Its operator has local-cluster control-plane privileges and should not be installed on a production cluster without an independent security and RBAC review.

## Deployment Profiles

| Profile | Namespaced visibility | Cluster-scoped visibility | Recommended use |
| --- | --- | --- | --- |
| `gitops/components/operator` | All namespaces | Polykube `ClusterMember` and `Federation` resources | Local demo or clusters where Polykube is trusted to manage workloads in every namespace |
| `gitops/overlays/operator-namespace-scoped` | Only `polykube-workloads` by default | Polykube `ClusterMember` and `Federation` resources | Clusters where all Polykube-managed applications can be isolated in one namespace |

The namespace-scoped profile passes `--watch-namespace=<namespace>` to controller-runtime. This limits its cache and reconciliation of namespaced Polykube resources, Secrets, ConfigMaps, Deployments, and Services. Cluster-scoped resources remain visible because the infrastructure API is cluster-scoped.

Do not deploy the restricted RBAC without the matching `--watch-namespace` argument. RBAC prevents unauthorized API operations, while cache scoping prevents controllers from queuing namespaced resources they are not intended to reconcile.

## Required Permissions

Both profiles grant only these controller operations:

| Resource | Scope | Access | Reason |
| --- | --- | --- | --- |
| `ClusterMember`, `Federation` | Cluster | Read; update status | Select the local member and report infrastructure readiness |
| `Workload` | Namespaced | Read; update status | Reconcile local workload intent and report target state |
| `ServiceEndpoint`, `DatastoreBinding` | Namespaced | Read; update finalizers and status | Reconcile routing/data intent and perform owned-object cleanup |
| `Secret`, `ConfigMap` | Namespaced | Read only | Validate workload references and read datastore connection data |
| `Deployment` | Namespaced | Read, watch, create, update | Manage Workload-owned Deployments and inject datastore environment values |
| `Service` | Namespaced | Read, watch, create, update, patch, delete | Manage Workload-owned Services and routing annotations |
| `Lease` | `polykube-system` | Create, read, update | Controller leader election |

The operator does not need permission to create or modify Secrets or ConfigMaps. It does not emit Kubernetes Events, so the manifests do not grant Event write access. It refuses to modify same-name Deployments or Services that are not controlled by the referenced Workload.

## Secret Trust Boundary

The operator ServiceAccount can read the full contents of every Secret in its watch scope because Kubernetes list/watch responses are not metadata-only. Restrict access to operator logs, process memory, debug endpoints, and the ServiceAccount token accordingly.

Workload image-pull and `envFrom` references remain Kubernetes object references in the generated Deployment. A `DatastoreBinding` is different: the controller reads the selected URL from its connection Secret and writes that URL as a literal environment value in the generated Deployment. Anyone who can read that Deployment can therefore read the injected connection URL.

Polykube does not create, rotate, encrypt, or replicate Secrets. Provision them independently in each member cluster, preferably through a dedicated secrets controller with narrowly scoped identity. Never commit plaintext Secret values to the GitOps repository.

## Namespace-Scoped Limitations

The supplied restricted profile supports one workload namespace per operator instance. All `Workload`, `ServiceEndpoint`, and `DatastoreBinding` objects and their referenced Secrets, ConfigMaps, Deployments, and Services must be in that namespace. Explicit references to another namespace fail because that namespace is outside both the cache and RBAC boundary.

Running multiple restricted operator instances in one cluster is not currently documented or tested. Their cluster-scoped infrastructure controllers could compete to update the same status objects. Use the default cluster-wide profile if one operator must manage multiple workload namespaces, and review its broader Secret and workload access before installation.

## Verification

Render both profiles before promotion:

```bash
kubectl kustomize gitops/components/operator
kubectl kustomize gitops/overlays/operator-namespace-scoped
bash scripts/validate-repo.sh
```

After applying the restricted profile, verify its effective permissions:

```bash
kubectl auth can-i get secrets \
  --as=system:serviceaccount:polykube-system:polykube-operator \
  --namespace=polykube-workloads
kubectl auth can-i get secrets \
  --as=system:serviceaccount:polykube-system:polykube-operator \
  --namespace=default
kubectl auth can-i update deployments.apps \
  --as=system:serviceaccount:polykube-system:polykube-operator \
  --namespace=polykube-workloads
```

The expected answers are `yes`, `no`, and `yes`. Also reconcile a Workload that references a local Secret and ConfigMap, then confirm its owned Deployment and Service become ready. Repeat the negative checks for every namespace that contains credentials or workloads outside Polykube's ownership boundary.
