# Grafana Pyroscope — using chart defaults for storage/config; the bundled
# Grafana Alloy agent (enabled by default) handles pod discovery via
# profiles.grafana.com/* annotations, not a custom scrape config.
pyroscope:
  persistence:
    enabled: true
    accessModes:
      - ReadWriteOnce
    size: 10Gi

  resources:
    requests:
      cpu: 100m
      memory: 256Mi

serviceMonitor:
  enabled: true