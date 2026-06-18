# AWS/GCP Example Root

This root demonstrates how AWS and GCP cluster bootstrap outputs can be converted into Polykube manifests.

It is an example, not a required product assumption. It does not configure cloud providers, create clusters, or read from a credential manager. Pass sanitized cluster outputs in through variables from your own infrastructure workflow.

## Usage

```bash
tofu init
tofu plan \
  -var aws_api_endpoint=https://aws.example.invalid \
  -var gcp_api_endpoint=https://gcp.example.invalid
```

Review the `manifests` output before handing it to GitOps.
