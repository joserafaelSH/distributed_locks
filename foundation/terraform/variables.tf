variable "kubeconfig_path" {
  description = "Path to kubeconfig written by k3d/kind"
  type        = string
  default     = "~/.kube/config"
}

variable "kube_context" {
  description = "kubeconfig context to use (k3d prefixes with k3d-<cluster-name>)"
  type        = string
  default     = "k3d-platform-lab"
}

variable "observability_namespace" {
  description = "Namespace for the whole observability stack"
  type        = string
  default     = "observability"
}

variable "apps_namespace" {
  description = "Namespace application workloads (service template, etc.) deploy into"
  type        = string
  default     = "apps"
}

variable "infra_namespace" {
  description = "Namespace for infrastructure components (Redis, Kafka, etc.)"
  type        = string
  default     = "infra"
}

variable "grafana_admin_password" {
  description = "Grafana admin password (change for anything beyond local dev)"
  type        = string
  default     = "admin"
  sensitive   = true
}

variable "postgres_admin_password" {
  description = "PostgreSQL postgres user password (change for anything beyond local dev)"
  type        = string
  default     = "postgres"
  sensitive   = true
}

variable "redis_password" {
  description = "Redis auth password (change for anything beyond local dev)"
  type        = string
  default     = "redis"
  sensitive   = true
}

variable "storage_class" {
  description = "StorageClass to use for PVCs (k3d ships 'local-path' by default)"
  type        = string
  default     = "local-path"
}

variable "retention_hours" {
  description = "Metrics/logs/traces retention, in hours, for local dev"
  type        = number
  default     = 72
}