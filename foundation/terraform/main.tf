module "namespaces" {
  source = "./modules/namespaces"

  observability_namespace = var.observability_namespace
  apps_namespace           = var.apps_namespace
}

module "observability" {
  source = "./modules/observability"

  namespace               = var.observability_namespace
  grafana_admin_password  = var.grafana_admin_password
  storage_class           = var.storage_class
  retention_hours         = var.retention_hours

  depends_on = [module.namespaces]
}