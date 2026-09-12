# OpenTelemetry Collector — the single ingest point every service ships
# OTLP traces + metrics to.
mode: deployment
replicaCount: 1

image:
  repository: otel/opentelemetry-collector-contrib

config:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318

  processors:
    batch: {}
    memory_limiter:
      check_interval: 5s
      limit_mib: 400
    resource:
      attributes:
        - key: collector.name
          value: shared-otel-collector
          action: upsert

  exporters:
    otlp/tempo:
      endpoint: "${tempo_endpoint}"
      tls:
        insecure: true
    prometheusremotewrite:
      endpoint: "http://${prometheus_endpoint}/api/v1/write"
    debug:
      verbosity: basic

  service:
    pipelines:
      traces:
        receivers: [otlp]
        processors: [memory_limiter, resource, batch]
        exporters: [otlp/tempo, debug]
      metrics:
        receivers: [otlp]
        processors: [memory_limiter, resource, batch]
        exporters: [prometheusremotewrite, debug]

ports:
  otlp:
    enabled: true
    containerPort: 4317
    servicePort: 4317
    protocol: TCP
  otlp-http:
    enabled: true
    containerPort: 4318
    servicePort: 4318
    protocol: TCP

resources:
  requests:
    cpu: 100m
    memory: 200Mi
  limits:
    memory: 500Mi