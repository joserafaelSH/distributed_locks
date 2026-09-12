variable "namespace" {
  type = string
}

variable "grafana_admin_password" {
  type      = string
  sensitive = true
}

variable "storage_class" {
  type = string
}

variable "retention_hours" {
  type = number
}