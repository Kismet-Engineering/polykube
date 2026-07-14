# End-to-End Validation — v0.1.0-alpha.1

This guide is the clean-machine quickstart gate required before cutting the release tag. Run every step in order on a machine with no prior repository state. Record the output of steps marked **[RECORD]** for inclusion in the release notes.

---

## 1. Prerequisites

Install all of the following before starting.

| Tool | Notes |
|---|---|
| Git | OS package manager |
| Docker | Docker Desktop / Colima / OrbStack — any compatible runtime |
| `mise` | `curl https://mise.run \| sh` — manages Go, kubectl, colima versions declared in `.mise.toml` |

Once `mise` is installed, run:

```bash
mise install
```

This provisions Go, kubectl, and colima at the exact versions declared in `.mise.toml`. No manual installs needed for those tools.

On macOS, start the Docker runtime before proceeding:

```bash
colima start --cpu 4 --memory 8 --disk 60
```

Verify:

```bash
git --version
docker info    # must show a running daemon
mise run --help
```

On macOS with Colima, start the runtime before proceeding:

```bash
colima start --cpu 4 --memory 8 --disk 60
```

---

## 2. Clone

```bash
git clone https://github.com/Kismet-Engineering/polykube
cd polykube
```

---

## 3. Repository Validation **[RECORD]**

```bash
bash scripts/validate-repo.sh
```

Expected output ends with:

```
repository validation passed
```

If it fails, stop and fix the failure before continuing.

---

## 4. Create Local Clusters

```bash
mise run local:cluster:create -- --clusters alpha,beta --workers 0
mise run local:cluster:status
```

Expected: two clusters named `alpha` and `beta` listed as running.

Export kubeconfigs so `kubectl` can reach both:

```bash
export KUBECONFIG=$(ls -1 examples/local-multicluster/state/kubeconfigs/*.yaml | paste -sd: -)
kubectl config get-contexts
```

Expected: two contexts — `polykube-alpha` and `polykube-beta`.

---

## 5. Install Cilium and Connect ClusterMesh **[RECORD]**

```bash
mise run local:cilium:preflight   -- --clusters alpha,beta
mise run local:cilium:install     -- --clusters alpha,beta
mise run local:cilium:clustermesh:enable  -- --clusters alpha,beta --service-type NodePort
mise run local:cilium:clustermesh:connect -- --source alpha --destination beta
mise run local:cilium:verify      -- --source alpha --destination beta
```

Expected: `cilium:verify` exits 0 with a connectivity report showing cross-cluster reachability.

```bash
mise run local:cilium:global-service:probe -- --source alpha --destination beta
```

Expected: probe exits 0, confirming that a pod in `alpha` can reach a global service served by `beta`.

---

## 6. Build and Deploy the Operator **[RECORD]**

Build the container image (Go compiles inside Docker — no local Go install needed):

```bash
mise run operator:image:build -- --image polykube-operator:dev
```

Load the image into both cluster runtimes and deploy:

```bash
mise run local:operator:image:load -- --clusters alpha,beta --image polykube-operator:dev
mise run local:operator:deploy    -- --clusters alpha,beta --image polykube-operator:dev
```

Verify the operator pods are running on each cluster:

```bash
kubectl --context polykube-alpha -n polykube-system get pods
kubectl --context polykube-beta  -n polykube-system get pods
```

Expected: one `polykube-operator-*` pod per cluster in `Running` state.

Confirm `--cluster-member-name` is set correctly in each operator Deployment:

```bash
kubectl --context polykube-alpha -n polykube-system get deploy polykube-operator -o jsonpath='{.spec.template.spec.containers[0].args}' | tr ',' '\n'
kubectl --context polykube-beta  -n polykube-system get deploy polykube-operator -o jsonpath='{.spec.template.spec.containers[0].args}' | tr ',' '\n'
```

Expected: alpha shows `--cluster-member-name=alpha`, beta shows `--cluster-member-name=beta`.

---

## 7. Apply Sample Demo Manifests

```bash
mise run local:demo:apply
```

This applies to each cluster:

- `clustermember-{alpha,beta}.yaml`
- `federation.yaml` (`local-dev` federation, selects env=dev members)
- `workload-echo.yaml` (hashicorp/http-echo, port 5678)
- `serviceendpoint-echo.yaml` (ActiveActive routing)

---

## 8. Verify ClusterMember **[RECORD]**

```bash
kubectl --context polykube-alpha get clustermember alpha -o yaml
kubectl --context polykube-beta  get clustermember beta  -o yaml
```

Expected on each: `Ready: "True"` condition, `observedGeneration` set, `lastObservedAt` set.

---

## 9. Verify Federation **[RECORD]**

```bash
kubectl --context polykube-alpha get federation local-dev -o yaml
```

Expected: `readyMembers: 2`, `members` list contains both `alpha` and `beta` with `ready: true`, `Ready: "True"` condition.

---

## 10. Verify Workload **[RECORD]**

Allow 30–60 seconds for pods to schedule after applying manifests.

```bash
kubectl --context polykube-alpha -n default get workload echo -o yaml
kubectl --context polykube-beta  -n default get workload echo -o yaml
```

Expected on each:

- `status.targets[0].clusterMemberRef`: `alpha` (or `beta`)
- `status.targets[0].state`: `Available` (once pod is running) or `Reconciling` (while pending)
- `status.conditions`: `RuntimeObjectsApplied: True`, `Available: True`

Check the underlying Deployment and pod:

```bash
kubectl --context polykube-alpha -n default get deploy echo
kubectl --context polykube-alpha -n default get pods -l polykube.dev/workload=echo

kubectl --context polykube-beta -n default get deploy echo
kubectl --context polykube-beta -n default get pods -l polykube.dev/workload=echo
```

---

## 11. Verify Service Annotations **[RECORD]**

```bash
kubectl --context polykube-alpha -n default get svc echo -o yaml | grep 'cilium\|annotations' -A5
kubectl --context polykube-beta  -n default get svc echo -o yaml | grep 'cilium\|annotations' -A5
```

Expected on both clusters:

```yaml
annotations:
  service.cilium.io/global: "true"
  service.cilium.io/shared: "true"
```

---

## 12. Cross-Cluster Routing Probe **[RECORD]**

Send a request from a pod in `alpha` to the echo service and confirm it resolves (responses may come from either cluster):

```bash
kubectl --context polykube-alpha -n default run probe --rm -i --restart=Never \
  --image=curlimages/curl -- curl -s --max-time 10 http://echo:5678
```

Expected: an HTTP response body from the echo server (e.g. `hello-world`). The important signal is that the request resolves — responses may come from either cluster's pod.

---

## 13. Render GitOps Component

Confirm the kustomize output renders cleanly:

```bash
kubectl kustomize gitops/components/operator
```

Expected: the default Deployment image is the published alpha image from `gitops/components/operator/deployment.yaml`, not the local development image.

Expected: YAML output with no errors.

---

## 14. Cleanup (Optional)

```bash
mise run local:cluster:delete -- --clusters alpha,beta
```

---

## Checklist After Validation

Once all steps above pass, update `docs/release/public-alpha-checklist.md` and proceed with:

1. Check `[ ] Quickstart is validated from a clean machine...`
2. Push the release tag: `git tag v0.1.0-alpha.1 && git push origin v0.1.0-alpha.1`
3. CI publishes `ghcr.io/kismet-engineering/polykube-operator:0.1.0-alpha.1`
4. Confirm `gitops/components/operator/deployment.yaml` points at the published tag
5. Create the GitHub release using `docs/release/v0.1.0-alpha.1-release-notes.md`
6. Check `[ ] Public release tag and release notes are reviewed`
7. Change repository visibility to public
8. Check `[ ] Repository visibility change is approved by the maintainer`
