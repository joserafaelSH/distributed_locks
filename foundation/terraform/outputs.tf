output "observability_namespace" {
  value = module.namespaces.observability_namespace
}

output "apps_namespace" {
  value = module.namespaces.apps_namespace
}

output "infra_namespace" {
  value = module.namespaces.infra_namespace
}

output "redis_service" {
  value = module.infra.redis_service_name
}

output "postgres_service" {
  value = module.infra.postgres_service_name
}

output "prometheus_service" {
  value = module.observability.prometheus_service_name
}

output "loki_service" {
  value = module.observability.loki_service_name
}

output "tempo_service" {
  value = module.observability.tempo_service_name
}

output "otel_collector_service" {
  value = module.observability.otel_collector_service_name
}

output "pyroscope_service" {
  value = module.observability.pyroscope_service_name
}