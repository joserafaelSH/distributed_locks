# kube-prometheus-stack values — local-dev sized (no HA, short retention)
prometheus:
  prometheusSpec:
    retention: "${retention}"
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: "${storage_class}"
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi
    resources:
      requests:
        cpu: 200m
        memory: 512Mi
    serviceMonitorSelectorNilUsesHelmValues: false
    podMonitorSelectorNilUsesHelmValues: false
    ruleSelectorNilUsesHelmValues: false
    enableRemoteWriteReceiver: true

alertmanager:
  enabled: true
  alertmanagerSpec:
    storage:
      volumeClaimTemplate:
        spec:
          storageClassName: "${storage_class}"
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 2Gi

grafana:
  adminPassword: "${admin_password}"
  persistence:
    enabled: true
    storageClassName: "${storage_class}"
    size: 2Gi
  sidecar:
    datasources:
      enabled: true
      label: grafana_datasource
      labelValue: "1"
    dashboards:
      enabled: true
      label: grafana_dashboard
      labelValue: "1"
  service:
    type: NodePort
    nodePort: 30300
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
        - name: default
          orgId: 1
          folder: ""
          type: file
          disableDeletion: false
          editable: true
          options:
            path: /var/lib/grafana/dashboards/default
  dashboards:
    default:
      go-runtime:
        gnetId: 6671
        revision: 1
        datasource: Prometheus
      k8s-cluster:
        gnetId: 315
        revision: 3
        datasource: Prometheus

kubeControllerManager:
  enabled: false
kubeEtcd:
  enabled: false
kubeScheduler:
  enabled: false