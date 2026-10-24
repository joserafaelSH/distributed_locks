variable "namespace" {
  type = string
}

variable "postgres_admin_password" {
  type      = string
  sensitive = true
}

variable "redis_password" {
  type      = string
  sensitive = true
}
