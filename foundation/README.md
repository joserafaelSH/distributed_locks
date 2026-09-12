# Foundation — Overview & Commands

## What this is

A local Kubernetes platform for practicing production-grade patterns —
IaC, full observability, no shortcuts — before any application code exists.

- **Cluster**: `k3d` (3 nodes: 1 control-plane, 2 workers), running as
  Docker containers, defined in `cluster/k3d-config.yaml`.
- **IaC**: Terraform (`kubernetes` + `helm` providers) manages every cluster
  resource — namespaces and Helm releases — as code. No manual `helm
  install` commands; everything is `terraform apply`/`destroy`.
- **Observability stack**, deployed via Helm through Terraform, wired
  before any app exists:
  | Pillar | Tool |
  |---|---|
  | Metrics | Prometheus + Grafana |
  | Logs | Loki + Promtail |
  | Traces | Tempo + OpenTelemetry Collector |
  | Profiles | Grafana Pyroscope |

**Design principle**: everything that touches the cluster is declared in
Terraform/Helm values files, not typed as one-off CLI commands — so the
whole environment is reproducible from a clean checkout.

**Status**: fully deployed and verified end-to-end (metrics, logs, traces,
and profiles all confirmed flowing through Grafana with a real service
running in-cluster). See `foundation-setup-explained.md` for the full
build log including every config issue hit and how it was resolved.

---

## What any service running on this cluster needs to know

If you're deploying a service into this cluster, here's the contract it
needs to follow to get observability "for free":

- **Traces + metrics**: send OTLP to
  `otel-collector-opentelemetry-collector.observability:4317` (in-cluster
  DNS — this is the *only* address that resolves; `localhost` does not
  work from inside a pod).
- **Logs**: just write structured JSON to stdout. Promtail tails every
  pod automatically — no per-service config needed. Include a `trace_id`
  field if you want log↔trace correlation in Grafana.
- **Profiling**: Pyroscope's bundled Alloy agent auto-discovers pods via
  annotations — **`profiles.grafana.com/*`**, not `pyroscope.io/*`. This
  was a real mistake we made and corrected; see the troubleshooting table
  below. Minimum required annotations on the pod template:
  ```yaml
  profiles.grafana.com/cpu.scrape: "true"
  profiles.grafana.com/cpu.port_name: "<name-of-your-pprof-port>"
  ```
  (repeat for `memory` / `goroutine` as needed) — the `port_name` must
  match a **named** container port, not a raw port number.

---

## Must-know commands

### Cluster lifecycle
```bash
k3d cluster create --config cluster/k3d-config.yaml   # create the cluster
k3d cluster delete platform-lab                       # tear it down
k3d cluster list                                      # check status
k3d kubeconfig write platform-lab                     # re-write kubeconfig if needed
k3d image import <image>:<tag> -c platform-lab         # import a locally-built image into the cluster
kubectl config current-context                        # confirm you're pointed at k3d-platform-lab
```

### Cluster inspection
```bash
kubectl get nodes                                      # confirm nodes are Ready
kubectl get pods -n observability                       # check stack health
kubectl get pods -n observability -w                    # watch live during a deploy
kubectl describe pod <name> -n observability             # debug Pending/CrashLoopBackOff (see Events)
kubectl logs <pod> -n observability                       # see why a container is crashing
kubectl get pvc -n observability                         # confirm storage bound (not stuck Pending)
kubectl get namespaces
```

### Terraform (IaC)
```bash
cd terraform
terraform init              # download providers / register new modules
terraform plan               # dry run — review before applying
terraform apply              # apply changes (prompts for `yes`)
terraform destroy            # tear down everything Terraform manages
terraform output             # print all outputs (service names, etc.)
terraform validate           # check config syntax without touching the cluster
```

### Helm (mostly for debugging under Terraform)
```bash
helm list -n observability                     # see release status (deployed/failed)
helm show values <repo>/<chart>                # inspect a chart's real default schema
                                                # — check this BEFORE writing custom values
helm uninstall <release> -n observability      # clean up a failed release before re-apply
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
```

### Accessing dashboards locally
```bash
kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80
kubectl -n observability port-forward svc/loki 3100:3100
kubectl -n observability port-forward svc/tempo 3100:3100
kubectl -n observability port-forward svc/kube-prometheus-stack-prometheus 9090:9090
kubectl -n observability port-forward svc/pyroscope 4040:4040
kubectl -n observability port-forward svc/otel-collector-opentelemetry-collector 4317:4317
```
Grafana: `http://localhost:3000` (admin/admin by default)

**Manually adding a datasource in Grafana?** Use the in-cluster DNS name,
not `localhost` — Grafana runs inside the cluster and can't see your host
machine's ports. E.g. Tempo's query API is
`http://tempo.observability.svc.cluster.local:3100` (short form `http://tempo:3100`
also works from within the same namespace). This is exactly the mistake
that produces a "Bad Gateway" error when adding a datasource manually.

> A Terraform-managed `kubernetes_config_map` (`grafana_extra_datasources`)
> exists in the observability module to auto-register Loki/Tempo/Pyroscope
> as datasources via Grafana's sidecar — apply it to avoid manual
> datasource setup entirely on a fresh cluster.

### Quick health checks (no browser needed)
```bash
curl http://localhost:3000/api/health                              # Grafana alive?
curl "http://localhost:3100/loki/api/v1/label/service_name/values"  # logs actually ingesting?
```

---

## Troubleshooting quick-reference

| Symptom | Likely cause | Check |
|---|---|---|
| `kubectl` "connection refused" via `host.docker.internal` | API server bound to `127.0.0.1` only | `kubeAPI.hostIP: "0.0.0.0"` in k3d config |
| Helm release stuck `failed` after `terraform apply` | Chart's own config validation rejected the values | `kubectl logs <pod>` — the real error is almost always there |
| Pod `Pending` | Storage/scheduling issue | `kubectl describe pod` → Events; `kubectl get pvc` |
| Pod `CrashLoopBackOff` | Bad config, not a resource issue | `kubectl logs <pod>` |
| "assumed config schema was wrong" | Didn't check the chart's actual defaults first | `helm show values <repo>/<chart>` before writing custom values |
| `kubectl port-forward` fails: "Only one usage of each socket address..." (Windows) | Something already bound to that local port — often a leftover `kubectl port-forward` from an earlier terminal | `netstat -ano \| findstr <port>`, kill the PID, or just forward to a different local port (e.g. `4318:4317` instead of `4317:4317`) |
| Grafana datasource "Bad Gateway" | Used `localhost` instead of the in-cluster DNS name | Use `http://<service>.<namespace>.svc.cluster.local:<port>` (or short form within the same namespace) |
| Pyroscope shows no profiles for a running, correctly-annotated pod | Query filters not selecting a profile type, or Alloy hasn't scraped yet | Clear/check filters in Explore first; wait one scrape interval (~15-60s) after deploy |
| Loki `CONFIG ERROR: invalid compactor config` | `compactor.retention_enabled: true` requires an explicit delete-request store | Add `compactor.delete_request_store: filesystem` |
| Service duplicate-port error on `helm_release` for the OTel Collector | Custom port key name didn't match the chart's actual default key, so it added a port instead of overriding one | Use the chart's real default port names (check via `helm show values`) rather than inventing new ones |
