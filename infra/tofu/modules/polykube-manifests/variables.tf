variable "federation_name" {
  description = "Name for the generated Polykube Federation resource."
  type        = string
}

variable "routing_mode" {
  description = "Federation routing mode."
  type        = string
  default     = "ActiveActive"

  validation {
    condition     = contains(["ActivePassive", "ActiveActive"], var.routing_mode)
    error_message = "routing_mode must be ActivePassive or ActiveActive."
  }
}

variable "networking_substrate" {
  description = "Descriptive name of the multicluster networking substrate."
  type        = string
  default     = ""
}

variable "networking_details" {
  description = "Optional substrate-specific metadata for the Federation networking block."
  type        = map(string)
  default     = {}
}

variable "members" {
  description = "Cluster members keyed by desired ClusterMember resource name."
  type = map(object({
    provider            = string
    region              = string
    zone                = string
    environment         = string
    cluster_name        = string
    api_endpoint        = string
    pod_cidr            = string
    service_cidr        = string
    labels              = map(string)
  }))
}
