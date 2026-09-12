locals {
  retention = "${var.retention_hours}h"
}

# ---------------------------------------------------------------------------
# Metrics: Prometheus + Grafana (kube-prometheus-stack bundles both, plus
# Alertmanager and the standard k8s ServiceMonitor CRDs)
# ---------------------------------------------------------------------------
resource "helm_release" "kube_prometheus_stack" {
  name             = "kube-prometheus-stack"
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  version          = "62.3.1"
  namespace        = var.namespace
  create_namespace = false
  timeout          = 600

  values = [
    templatefile("${path.module}/values/prometheus-values.yaml.tpl", {
      storage_class  = var.storage_class
      retention      = local.retention
      admin_password = var.grafana_admin_password
    })
  ]
}

output "grafana_service_name" {
  value = "kube-prometheus-stack-grafana"
}

output "prometheus_service_name" {
  value = "kube-prometheus-stack-prometheus"
}


# ---------------------------------------------------------------------------
# Logs: Loki (single-binary mode for local dev) + Promtail (DaemonSet)
# ---------------------------------------------------------------------------
resource "helm_release" "loki" {
  name       = "loki"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "loki"
  version    = "6.6.4"
  namespace  = var.namespace
  timeout    = 300

  values = [
    templatefile("${path.module}/values/loki-values.yaml.tpl", {
      storage_class   = var.storage_class
      retention_hours = var.retention_hours
    })
  ]

  depends_on = [helm_release.kube_prometheus_stack]
}

resource "helm_release" "promtail" {
  name       = "promtail"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "promtail"
  version    = "6.16.4"
  namespace  = var.namespace
  timeout    = 300

  values = [
    file("${path.module}/values/promtail-values.yaml")
  ]

  depends_on = [helm_release.loki]
}

output "loki_service_name" {
  value = "loki"
}


# ---------------------------------------------------------------------------
# Traces: Tempo (receiver + storage) + OpenTelemetry Collector (the single
# ingest point every service ships OTLP to — traces AND metrics)
# ---------------------------------------------------------------------------
resource "helm_release" "tempo" {
  name       = "tempo"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "tempo"
  version    = "1.10.1"
  namespace  = var.namespace
  timeout    = 300

  values = [
    templatefile("${path.module}/values/tempo-values.yaml.tpl", {
      storage_class   = var.storage_class
      retention_hours = var.retention_hours
    })
  ]

  depends_on = [helm_release.kube_prometheus_stack]
}

resource "helm_release" "otel_collector" {
  name       = "otel-collector"
  repository = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart      = "opentelemetry-collector"
  version    = "0.108.0"
  namespace  = var.namespace
  timeout    = 300

  values = [
    templatefile("${path.module}/values/otel-collector-values.yaml.tpl", {
      tempo_endpoint      = "tempo.${var.namespace}.svc.cluster.local:4317"
      prometheus_endpoint = "kube-prometheus-stack-prometheus.${var.namespace}.svc.cluster.local:9090"
    })
  ]

  depends_on = [helm_release.tempo, helm_release.kube_prometheus_stack]
}

output "tempo_service_name" {
  value = "tempo"
}

output "otel_collector_service_name" {
  value = "otel-collector-opentelemetry-collector"
}

# ---------------------------------------------------------------------------
# Continuous profiling: Grafana Pyroscope (scrapes pprof from any pod
# annotated pyroscope.io/scrape=true — the service template sets this)
# ---------------------------------------------------------------------------
resource "helm_release" "pyroscope" {
  name       = "pyroscope"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "pyroscope"
  version    = "1.9.1"
  namespace  = var.namespace
  timeout    = 300

  values = [
    file("${path.module}/values/pyroscope-values.yaml.tpl")
  ]

  depends_on = [helm_release.kube_prometheus_stack]
}
output "pyroscope_service_name" {
  value = "pyroscope"
}