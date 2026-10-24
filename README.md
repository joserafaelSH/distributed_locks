# Distributed Locks

A from-scratch build of a distributed locking service, done the way a real production system
would be: infrastructure, observability, and service plumbing come first, as code, before any
application logic exists. No shortcuts, no manual cluster setup, no silently-missing telemetry.

## Why this exists

Distributed locks are a classic hard problem (Redlock, fencing tokens, lease expiry, split-brain
under network partitions) — but shipping one safely also means the system *around* it has to be
right: reproducible infra, full observability, and a service skeleton that doesn't need to be
re-plumbed for every new project. This repo builds that foundation first, verifies it end-to-end,
then builds the lock service on top of it.

## Structure

| Path | What it is |
|---|---|
| [`foundation/`](foundation) | Local Kubernetes platform (`k3d`) provisioned entirely via Terraform, with a full observability stack (Prometheus/Grafana, Loki, Tempo, Pyroscope) wired in before any app is deployed. |
| [`go-service-template/`](go-service-template) | The reusable Go service skeleton every service in this project starts from — HTTP + gRPC servers, OTel tracing, structured logging, pprof profiling, and a Helm chart — pre-wired to the foundation's observability stack. |

## Status

Foundation and service template are built and verified end-to-end (metrics, logs, traces, and
profiles all confirmed flowing through Grafana from a real pod). The distributed lock service
itself — built on top of this template, using Redis/Postgres for coordination — is the next layer
to land.

See each directory's own README for setup commands and troubleshooting notes.
