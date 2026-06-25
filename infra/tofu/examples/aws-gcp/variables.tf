variable "federation_name" {
  description = "Name for the example Federation resource."
  type        = string
  default     = "aws-gcp"
}

variable "environment" {
  description = "Environment label for generated ClusterMember resources."
  type        = string
  default     = "dev"
}

variable "aws_api_endpoint" {
  description = "Kubernetes API endpoint for the AWS example cluster."
  type        = string
}

variable "gcp_api_endpoint" {
  description = "Kubernetes API endpoint for the GCP example cluster."
  type        = string
}

variable "aws_region" {
  description = "AWS region label for the example ClusterMember."
  type        = string
  default     = "us-east-1"
}

variable "gcp_region" {
  description = "GCP region label for the example ClusterMember."
  type        = string
  default     = "us-central1"
}
