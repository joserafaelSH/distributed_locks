# Loki single-binary deployment — sufficient for local dev.
deploymentMode: SingleBinary

loki:
  auth_enabled: false
  commonConfig:
    replication_factor: 1
  storage:
    type: filesystem
  schemaConfig:
    configs:
      - from: "2024-01-01"
        store: tsdb
        object_store: filesystem
        schema: v13
        index:
          prefix: index_
          period: 24h
  limits_config:
    retention_period: ${retention_hours}h
    ingestion_rate_mb: 16
    ingestion_burst_size_mb: 32
  compactor:
    retention_enabled: true
    delete_request_store: filesystem

singleBinary:
  replicas: 1
  persistence:
    enabled: true
    storageClass: "${storage_class}"
    size: 10Gi

read:
  replicas: 0
write:
  replicas: 0
backend:
  replicas: 0

gateway:
  enabled: false

test:
  enabled: false

monitoring:
  serviceMonitor:
    enabled: true
  selfMonitoring:
    enabled: false
  lokiCanary:
    enabled: false