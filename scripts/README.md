# Scripts

Local helper scripts live here. Scripts must avoid organization-specific credentials, private domains, or hidden live-system assumptions.

## Operator Helpers

- `operator_render.sh`: render the GitOps operator component with an image override.
- `operator_deploy.sh`: apply CRDs and operator runtime manifests to a Kubernetes context.
- `operator_undeploy.sh`: remove operator runtime manifests and optionally CRDs from a Kubernetes context.
