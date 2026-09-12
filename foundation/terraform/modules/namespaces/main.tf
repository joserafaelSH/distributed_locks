variable "observability_namespace" {
  type = string
}

variable "apps_namespace" {
  type = string
}

resource "kubernetes_namespace" "observability" {
  metadata {
    name = var.observability_namespace
    labels = {
      "app.kubernetes.io/part-of" = "platform-lab"
      "purpose"                   = "observability"
    }
  }
}

resource "kubernetes_namespace" "apps" {
  metadata {
    name = var.apps_namespace
    labels = {
      "app.kubernetes.io/part-of" = "platform-lab"
      "purpose"                   = "application-workloads"
    }
  }
}

output "observability_namespace" {
  value = kubernetes_namespace.observability.metadata[0].name
}

output "apps_namespace" {
  value = kubernetes_namespace.apps.metadata[0].name
}