# Go Service Template — Overview & Commands

## What this is

The bootstrapped Go service every downstream project starts from, instead
of rebuilding the same plumbing each time. Ships with:

- **HTTP server** (chi router) with OTel tracing middleware and
  structured, trace-id-correlated request logging
- **gRPC server** with OTel tracing and the standard gRPC health-check
  service pre-registered
- **`net/http/pprof`** exposed on its own dedicated port, correctly
  annotated for Grafana Pyroscope's Alloy-based auto-discovery
- **Structured logging** (zerolog) — every log line carries `trace_id`/
  `span_id` automatically when written inside a traced request
- **Helm chart** to deploy it into the [foundation](../foundation) cluster

**Status**: builds, runs locally, deploys to the cluster, and has been
verified end-to-end — traces visible in Tempo, metrics in Prometheus,
logs in Loki, and CPU/memory/goroutine profiles in Pyroscope, all from
this exact template running as a real pod.

---

## Folder structure

```
go-service-template/
├── cmd/server/main.go        # entrypoint — wires everything together
├── internal/
│   ├── config/                # env var loading
│   ├── logger/                 # zerolog + trace-id correlation
│   ├── telemetry/               # OTel SDK setup (traces + metrics → Collector)
│   ├── server/                   # HTTP (chi) + gRPC + pprof servers
│   └── handlers/                  # /healthz, /readyz
├── helm/service-template/     # Helm chart to deploy this into the cluster
├── Dockerfile
├── go.mod / go.sum
```

---

## Local dev loop (no cluster needed)

```bash
go mod tidy          # resolve deps — only meaningful once source files exist;
                      # running this against an empty package tree wipes go.mod's
                      # require block, since it only keeps what's actually imported
go run ./cmd/server
```

Default ports: HTTP `8080`, gRPC `9090`, pprof `6060` — all overridable via
env vars (`HTTP_PORT`, `GRPC_PORT`, `PPROF_PORT`). If `8080` is already
taken locally (very likely if the foundation's k3d cluster is running —
its load balancer binds host port 8080 too), just pick a different one:

```bash
HTTP_PORT=8081 go run ./cmd/server
```

Sanity checks once it's running:
```bash
curl http://localhost:8081/healthz     # -> ok
curl http://localhost:8081/readyz      # -> ready
curl http://localhost:6060/debug/pprof/   # -> pprof index page
```

**To see real telemetry locally** (traces/metrics only — logs and
profiles require running in-cluster, see below), port-forward the shared
OTel Collector and point the app at it:
```bash
kubectl -n observability port-forward svc/otel-collector-opentelemetry-collector 4318:4317
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 HTTP_PORT=8081 go run ./cmd/server
```
Then generate traffic and check Grafana → Explore → Tempo/Prometheus.

---

## Building & deploying into the cluster

This is where Helm comes in — same pattern as everything in `foundation`,
except this one release you'll likely re-deploy often as you iterate on
the app, so it's worth understanding each step rather than just running
`terraform apply` for it.

### 1. Build the image
```bash
docker build -t service-template:latest .
```
Multi-stage build: compiles in a full `golang` image, then copies just the
static binary into a minimal `distroless` image — no shell, no package
manager, smaller attack surface, faster pulls.

### 2. Import it into k3d
k3d clusters can't see your local Docker daemon's images automatically —
this step is **required** every time you rebuild, or the pod will sit in
`ImagePullBackOff`:
```bash
k3d image import service-template:latest -c platform-lab
```

### 3. Deploy with Helm
```bash
helm upgrade --install service-template \
  helm/service-template \
  --namespace apps --create-namespace \
  --set image.repository=service-template \
  --set image.tag=latest
```
- `upgrade --install` — installs fresh if the release doesn't exist yet,
  upgrades in place if it does. This is the command you'll run every
  single time you rebuild, not just the first time.
- Path is **relative to wherever you run the command from** — if you're
  not sitting inside `go-service-template/`, adjust the path accordingly
  (this tripped us up once: running from a different directory produced
  a path like `v1\foundation\go-service-template` that didn't actually
  exist, since `helm` interpreted it relative to the wrong starting
  point).
- `--set image.tag=latest` — fine for local iteration; in a real pipeline
  you'd tag with a commit SHA and bump this per deploy so Kubernetes
  actually detects a change (with a static `latest` tag, a plain
  `helm upgrade` might not trigger a pod restart even after rebuilding
  the image — use `kubectl rollout restart deployment/service-template -n apps`
  if you rebuild without changing the tag).

### 4. Confirm it's healthy
```bash
kubectl get pods -n apps
kubectl logs -n apps deployment/service-template
```

### Iterating (steps 1–3 again after every code change)
```bash
docker build -t service-template:latest .
k3d image import service-template:latest -c platform-lab
kubectl rollout restart deployment/service-template -n apps
kubectl get pods -n apps -w
```

### Tearing down just this service
```bash
helm uninstall service-template -n apps
```

---

## What the Helm chart actually does

`helm/service-template/` is a minimal chart, not a generic one pulled
from a repo — it's shaped specifically for this template:

- **`values.yaml`** — the knobs: image repo/tag, ports, resource
  requests/limits, probe paths, and the OTel Collector endpoint
  (`otel-collector-opentelemetry-collector.observability:4317`, in-cluster
  DNS — pointing this at `localhost` will silently fail from inside a
  pod, since a pod has no route to your host machine).
- **`templates/deployment.yaml`** — the Deployment spec, including the
  **Pyroscope annotations** that let Grafana Alloy discover and scrape
  this pod for continuous profiling:
  ```yaml
  profiles.grafana.com/cpu.scrape: "true"
  profiles.grafana.com/cpu.port_name: "pprof"
  ```
  `port_name` must reference a **named** container port (`name: pprof`),
  not a raw port number — this is deliberately `profiles.grafana.com/*`,
  not `pyroscope.io/*`, because that's what Pyroscope's chart (in
  `foundation`) actually uses; we only found this out by inspecting the
  chart's real default values after an earlier assumption was wrong.
- **`templates/service.yaml`** — exposes HTTP/gRPC/pprof as named ports
  on a ClusterIP Service, so other things in-cluster (and `kubectl
  port-forward`) can reach them by name.
- **`templates/servicemonitor.yaml`** — optional (`serviceMonitor.enabled:
  false` by default); flip on if you add a `/metrics` HTTP endpoint and
  want Prometheus to pull-scrape it directly, as an alternative to the
  push-based OTLP metrics path.

Changing chart values without editing the file: use `--set`, e.g.
```bash
helm upgrade --install service-template helm/service-template \
  --namespace apps \
  --set replicaCount=2 \
  --set resources.requests.cpu=200m
```
or point at an override file with `-f my-overrides.yaml` for anything
more than a couple of one-off flags.

---

## Must-know Go gotchas (things that actually broke during the build)

| Issue | Fix |
|---|---|
| `cannot call pointer method Info on zerolog.Logger` | zerolog's methods use pointer receivers — you can't chain `.Info()` directly off a function call's return value. Assign to a variable first: `l := logger.FromContext(ctx); l.Info()...` |
| `go mod tidy` wiped `go.mod`'s `require` block | It only keeps dependencies actually imported by existing `.go` files — running it before any source code exists (or exists yet to use a given import) clears everything. Only run it once real code imports each dependency. |
| gRPC health check imports | `google.golang.org/grpc/health` and `.../health/grpc_health_v1` are sub-packages of the `google.golang.org/grpc` module already in `go.mod` — no separate `require` entry needed; `go mod tidy` resolves them from the existing dependency. |
| Local port `8080` bind failure | The foundation's k3d load balancer also binds host port 8080 (`cluster/k3d-config.yaml`). Run the app on a different `HTTP_PORT` for local dev, or stop the cluster first. |

---

## Quick reference: full loop, start to finish

```bash
# local dev
go run ./cmd/server

# ship to cluster
docker build -t service-template:latest .
k3d image import service-template:latest -c platform-lab
helm upgrade --install service-template helm/service-template \
  --namespace apps --create-namespace \
  --set image.repository=service-template --set image.tag=latest

# verify
kubectl get pods -n apps
kubectl -n apps port-forward svc/service-template-service-template 8080:8080
curl http://localhost:8080/healthz

# check telemetry in Grafana (localhost:3000)
#   Explore -> Loki:      {namespace="apps"}
#   Explore -> Tempo:     {resource.service.name="service-template"}
#   Explore -> Prometheus: http_server_...
#   Explore -> Pyroscope: select a profile type, clear filters, wait ~30s after deploy
```