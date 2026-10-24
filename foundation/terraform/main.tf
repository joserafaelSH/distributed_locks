module "namespaces" {
  source = "./modules/namespaces"

  observability_namespace = var.observability_namespace
  apps_namespace          = var.apps_namespace
  infra_namespace         = var.infra_namespace
}

module "observability" {
  source = "./modules/observability"

  namespace              = var.observability_namespace
  grafana_admin_password = var.grafana_admin_password
  storage_class          = var.storage_class
  retention_hours        = var.retention_hours

  depends_on = [module.namespaces]
}

module "infra" {
  source                  = "./modules/infra"
  namespace               = var.infra_namespace
  postgres_admin_password = var.postgres_admin_password
  redis_password          = var.redis_password
  depends_on              = [module.namespaces, module.observability]
}