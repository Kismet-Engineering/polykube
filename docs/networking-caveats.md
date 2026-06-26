# Cross-Cloud Networking Caveats

Polykube's routing model assumes two separate layers:

1. An underlay that makes every member cluster's pod CIDRs reachable from every other member cluster.
2. Cilium ClusterMesh for workload-level identity, service projection, and global-service routing.

This split is intentional. ClusterMesh does not replace inter-cluster IP reachability; it depends on it. The notes below come from real AWS/GCP validation and are worth checking before spending time debugging application behavior.

## GKE Dataplane V2

GKE Dataplane V2 is Google's managed Cilium datapath. It is not a drop-in replacement for self-managed Cilium when you need ClusterMesh control over the Cilium installation.

For GKE clusters that will run self-managed Cilium ClusterMesh:

- Create the cluster with the legacy datapath, for example `datapath_provider = "LEGACY_DATAPATH"` in Terraform-managed GKE configuration.
- Treat this as a cluster-create-time decision. GKE datapath provider changes are effectively rebuild work, not a safe day-two toggle.
- Install Cilium yourself with the GKE-specific settings your cluster needs, instead of trying to layer ClusterMesh on top of Dataplane V2.

## GKE Cilium Settings

GKE has a few Cilium-specific differences that are easy to miss:

- GKE stores CNI binaries under `/home/kubernetes/bin`, not `/opt/cni/bin`.
- If `kubeProxyReplacement` is enabled, set `k8sServiceHost` and `k8sServicePort` directly so Cilium and ClusterMesh components can reach the Kubernetes API before service-VIP routing is healthy.
- Native routing on GKE may need `directRoutingSkipUnreachable: true` because nodes can route through a gateway rather than being directly reachable at L2.
- For ClusterMesh global services, validate Cilium socket-LB coverage from normal workload pods. In one GKE rollout, service imports and BPF service maps existed, but GCP-to-AWS remote ClusterIP calls left the source pod as literal ClusterIP traffic until Cilium's cgroup/socket-LB attachment was repaired by enabling `cgroup.autoMount.enabled=true` and preserving the full known-good ClusterMesh Helm values.

Do not assume a healthy ClusterMesh control plane means global-service translation is working. Validate from workload pods.

## EKS Cilium Settings

On EKS, the default `aws-node` and `kube-proxy` DaemonSets conflict with a Cilium-owned ENI plus kube-proxy-replacement setup.

Before relying on Cilium ENI mode:

- Disable or remove `aws-node` before Cilium takes over pod IP management.
- Disable or remove `kube-proxy` when using `kubeProxyReplacement: true`.
- Use native routing with ENI IPAM; tunnel mode is incompatible with Cilium ENI IPAM.
- Verify pod egress through secondary ENIs, not only the primary interface. Masquerade interface patterns may need to include secondary ENIs.

## Underlay Route Advertisement

When clusters do not share private routing, a WireGuard underlay such as Netmaker can provide the required pod-CIDR reachability. The important operational primitive is route advertisement, not just VPN enrollment.

Route advertisement should be reconciled and observable:

- Advertise every member's pod CIDRs, and any service CIDRs that your validation path intentionally routes through the underlay.
- Track which gateway node advertises each CIDR.
- Detect stale gateway peers after node churn. A Cilium global service can have correct imported backends while the underlay route still points at a dead WireGuard peer, producing `No route to host` or timeouts.
- Reconcile pulled client routes, not only provider API state. API-level gateway assignment can look correct while local node route tables are stale.
- Keep route-sync logic as versioned, testable code where possible, not large scripts embedded in ConfigMaps.

On Linux nodes, also check reverse-path filtering for forwarded overlay traffic. Strict `rp_filter` on default-route or WireGuard interfaces can drop valid asymmetric return traffic. Loose mode is often required for Netmaker/WireGuard gateway forwarding.

## ClusterMesh Control Plane Drift

ClusterMesh peering can drift even when summaries look healthy.

Known failure patterns to guard against:

- Load balancer IP drift for a ClusterMesh API server endpoint can leave peer `hostAliases` or equivalent connection state stale.
- Mismatched or regenerated Cilium CAs can break remote kvstore connectivity unless both clusters trust the required CA bundle.
- Summary commands can report remote clusters ready while lower-level KVStoreMesh status still shows an initial connection failure.

Validation should include raw ClusterMesh state from both sides, not only a single summarized health command.

## Validation Matrix

Before declaring a cross-cloud substrate ready for workloads, run functional probes that separate the layers:

| Path | What It Proves |
| --- | --- |
| AWS pod to GCP pod IP | Underlay, routing, policy, return path |
| GCP pod to AWS pod IP | Underlay, routing, policy, return path in the reverse direction |
| AWS pod to GCP global service | ClusterMesh service import and source-side service translation |
| GCP pod to AWS global service | Reverse ClusterMesh service import and source-side service translation |
| Same-name service with `service.cilium.io/global=true` and `service.cilium.io/affinity=remote` from both clusters | Whether service-DNS peer discovery is actually bidirectional |

Treat direct pod-IP reachability and global-service reachability as different contracts. Direct pod traffic can be fully healthy while global-service translation is asymmetric or broken in one source direction.

## Troubleshooting Order

When cross-cluster traffic fails, avoid changing Cilium values speculatively. Narrow the layer first:

1. Confirm both clusters have current underlay routes for remote pod CIDRs.
2. Confirm direct pod-IP TCP checks in both directions.
3. Confirm ClusterMesh remote cluster and remote service state from both clusters.
4. From the failing source pod, test the remote global-service ClusterIP and record whether traffic is translated to a remote backend or leaves as literal ClusterIP traffic.
5. If direct pod paths pass but global-service paths fail, focus on Cilium service translation, cgroup/socket-LB attachment, service import state, or ClusterMesh CA/kvstore state rather than generic VPC or VPN routing.
