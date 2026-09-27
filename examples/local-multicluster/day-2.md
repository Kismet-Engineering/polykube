# Day-2 Walkthrough

This walkthrough continues from a working local demo. It changes the running `echo` Workload the way an operator would after first deploy, and shows what Polykube reports at each step.

Polykube is an experimental public alpha. Several behaviors below are intentionally shallow; each scenario ends with its limits.

## Before you start

Complete the [getting started guide](../../docs/getting-started.md) through [step 6](../../docs/getting-started.md#6-inspect-generated-resources), so that:

- the `polykube-alpha` and `polykube-beta` contexts are available through `KUBECONFIG`
- `mise run local:workload:status` shows `echo` as `Available` on both contexts

Day-2 manifests live in [`manifests/day-2/`](manifests/day-2/). In GitOps you would commit the same change and let Flux apply it to every member cluster. Here, a small helper applies a file to both clusters:

```bash
apply_both() {
  for ctx in polykube-alpha polykube-beta; do
    kubectl --context "${ctx}" apply -f "$1"
  done
}
cd examples/local-multicluster/manifests
```

Every scenario starts from, and returns to, the baseline manifests `workload-echo.yaml` and `serviceendpoint-echo.yaml`.

## 1. Update the image

Pin the image from `latest` to an explicit release:

```bash
apply_both day-2/workload-echo-pinned.yaml
mise run local:workload:status
```

**Expected outcome:**

- The `ACTIVE IMAGE` column shows `hashicorp/http-echo:1.0.0` for both contexts.
- Each target moves through `Reconciling` back to `Available` as the Deployment rolls out.
- To check one cluster directly:

  ```bash
  kubectl --context polykube-alpha -n default get workload echo -o jsonpath='{.status.activeImage}{"\n"}'
  kubectl --context polykube-alpha -n default rollout status deployment/echo
  ```

**Limits:** `status.activeImage` reports the image the operator applied, not an image digest. Target health is inferred only from the Deployment's `Available` condition. There is no canary or staged rollout across clusters: each cluster rolls out as soon as the change reaches it.

## 2. Change target members

Restrict `echo` to the `alpha` member:

```bash
apply_both day-2/workload-echo-alpha-only.yaml
mise run local:workload:status
```

**Expected outcome:**

- `polykube-alpha` stays `Available`.
- `polykube-beta` reports `Pending`, with a target message saying `beta` is excluded by `targetPolicy`:

  ```bash
  kubectl --context polykube-beta -n default get workload echo \
    -o jsonpath='{.status.conditions[?(@.type=="Pending")].reason}{"\n"}'
  # ExcludedByTargetPolicy
  ```

- **Beta's `echo` Deployment and Service are still running.** Excluding a member does not remove runtime objects that were created before the exclusion:

  ```bash
  kubectl --context polykube-beta -n default get deployment,service echo
  ```

To stop the workload on `beta` while it stays excluded, delete those objects by hand. The controller does not recreate them while `beta` is excluded:

```bash
kubectl --context polykube-beta -n default delete deployment,service echo
```

While `beta` has no `echo` Service, its `ServiceEndpoint` reports `Degraded/ServiceNotFound`.

Restore both members:

```bash
apply_both workload-echo.yaml
```

**Expected outcome:** both contexts return to `Available`, and `beta` gets a fresh Deployment and Service if you deleted them.

**Limits:** leaving runtime objects in place after an exclusion is current alpha behavior, not a final design. The decision is tracked in [#46](https://github.com/Kismet-Engineering/polykube/issues/46) and listed in [known limitations](../../docs/known-limitations.md#operator).

## 3. Recover from a missing dependency

Reference a Secret that does not exist yet:

```bash
apply_both day-2/workload-echo-envfrom.yaml
mise run local:workload:status
```

**Expected outcome:**

- Both targets report `Degraded`:

  ```bash
  kubectl --context polykube-alpha -n default get workload echo \
    -o jsonpath='{.status.conditions[?(@.type=="Degraded")].reason}{": "}{.status.conditions[?(@.type=="Degraded")].message}{"\n"}'
  # SecretNotFound: Secret "echo-config" not found in namespace "default". ...
  ```

- The existing Deployment keeps running its previous spec. Polykube does not apply a spec it cannot satisfy.

Create the Secret in each cluster. Polykube never copies Secrets between clusters:

```bash
apply_both day-2/secret-echo-config.yaml
```

**Expected outcome:**

- Within about 30 seconds (the retry interval), `Degraded` clears and targets return to `Reconciling`, then `Available`.
- The Deployment now references the Secret:

  ```bash
  kubectl --context polykube-alpha -n default get deployment echo \
    -o jsonpath='{.spec.template.spec.containers[0].envFrom[0].secretRef.name}{"\n"}'
  # echo-config
  ```

Return to the baseline:

```bash
apply_both workload-echo.yaml
```

**Limits:** missing dependencies are retried on a timer rather than watched, so recovery is not instant. The [recovery matrix](../../docs/architecture.md#reconciliation-failures-and-recovery) lists every reason and its fix, and the [secrets model](../../docs/architecture.md#secrets-model) describes how to provision Secrets per cluster.

## 4. Switch the active/passive primary

Move `echo` from active/active to active/passive with `alpha` as the primary:

```bash
apply_both day-2/serviceendpoint-echo-primary-alpha.yaml
for ctx in polykube-alpha polykube-beta; do
  kubectl --context "${ctx}" -n default get svc echo \
    -o jsonpath="{.metadata.annotations.service\.cilium\.io/shared}{\"  ${ctx}\n\"}"
done
```

**Expected outcome:**

- `true` on `polykube-alpha` and `false` on `polykube-beta`: only the primary shares its endpoints with the global service.
- `kubectl -n default get serviceendpoint echo -o jsonpath='{.status.activeMemberRef}'` returns `alpha` on both contexts.

Switch the primary to `beta`:

```bash
apply_both day-2/serviceendpoint-echo-primary-beta.yaml
```

**Expected outcome:** the annotations flip (`false` on alpha, `true` on beta), and `activeMemberRef` is `beta`.

**Limits:** Polykube only sets Cilium global-service annotations. Switching the primary is a manual, GitOps-driven change. There is no health-driven failover: `spec.failoverPolicy` is accepted but not acted on (see [API field support](../../docs/api-field-support.md#serviceendpoint)). As the [networking caveats](../../docs/networking-caveats.md) explain, confirm global-service translation independently of the annotations.

## 5. Roll back through Git

In a GitOps repository, rolling back means reverting the commit that introduced the change and letting Flux apply the previous manifests to every member cluster. Nothing in Polykube needs to be told about the rollback.

Locally, re-apply the baseline manifests:

```bash
apply_both workload-echo.yaml
apply_both serviceendpoint-echo.yaml
mise run local:workload:status
```

**Expected outcome:**

- The image is back to `hashicorp/http-echo:latest`, and every target is `Available`.
- The `echo` Service carries `service.cilium.io/shared: "true"` on both clusters again.

**Limits:** Polykube keeps no rollout history of its own. Git history is the audit trail. Progressive delivery and automated rollback belong to dedicated rollout controllers (see [decision 0003](../../docs/decisions/0003-crd-model-v0.md)).

## Automated coverage

CI runs the single-cluster scenarios from this walkthrough in `scripts/e2e.sh` on every pull request:

- a server-side dry-run of every manifest in `manifests/day-2/`
- image change and rollback
- missing-secret degradation and recovery
- exclusion of a running member (including the retained objects)
- the primary switch as seen from a non-primary member

Cross-cluster routing after a primary switch requires two clusters with ClusterMesh. For that, use the local release gate in [`README.md`](README.md#release-validation-gate).
