# Tempo — single-binary, filesystem-backed trace storage for local dev.
tempo:
  retention: ${retention_hours}h
  storage:
    trace:
      backend: local
      local:
        path: /var/tempo/traces
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318

persistence:
  enabled: true
  storageClassName: "${storage_class}"
  size: 10Gi

serviceMonitor:
  enabled: true

resources:
  requests:
    cpu: 100m
    memory: 256Mi