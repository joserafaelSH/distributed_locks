
# ---------------------------------------------------------------------------
# Resource: Helm Release for Redis
# ---------------------------------------------------------------------------
resource "helm_release" "redis" {
  name       = "redis"
  repository = "https://charts.bitnami.com/bitnami"
  chart      = "redis"
  version    = "17.3.7"
  namespace  = var.namespace

  set {
    name  = "architecture"
    value = "standalone"
  }

  set_sensitive {
    name  = "auth.password"
    value = var.redis_password
  }

  # Bitnami retired the free `bitnami/*` images on Docker Hub (Aug 2025);
  # the old tags this chart version pins now only exist under `bitnamilegacy/*`.
  set {
    name  = "image.repository"
    value = "bitnamilegacy/redis"
  }

  set {
    name  = "metrics.image.repository"
    value = "bitnamilegacy/redis-exporter"
  }

  set {
    name  = "volumePermissions.image.repository"
    value = "bitnamilegacy/os-shell"
  }

  set {
    name  = "metrics.enabled"
    value = "true"
  }

  set {
    name  = "metrics.serviceMonitor.enabled"
    value = "true"
  }

  set {
    name  = "master.resources.requests.cpu"
    value = "100m"
  }

  set {
    name  = "master.resources.requests.memory"
    value = "128Mi"
  }

  set {
    name  = "master.resources.limits.cpu"
    value = "250m"
  }

  set {
    name  = "master.resources.limits.memory"
    value = "256Mi"
  }
}

output "redis_service_name" {
  value = "redis-master"
}


resource "helm_release" "postgres" {
  name = "postgres"
  # Bitnami's classic https chart repo entry for this chart is broken;
  # this version is only served from their OCI registry.
  chart     = "oci://registry-1.docker.io/bitnamicharts/postgresql"
  version   = "18.8.9"
  namespace = var.namespace

  # Bitnami retired the free `bitnami/*` images on Docker Hub (Aug 2025);
  # same fix as the Redis release above.
  set {
    name  = "image.repository"
    value = "bitnamilegacy/postgresql"
  }

  set {
    name  = "metrics.image.repository"
    value = "bitnamilegacy/postgres-exporter"
  }

  set_sensitive {
    name  = "auth.postgresPassword"
    value = var.postgres_admin_password
  }

  set {
    name  = "auth.username"
    value = "myuser"
  }

  set {
    name  = "auth.database"
    value = "mydb"
  }

  # Passed as a values file (not `set`) because `--set` splits on commas,
  # which corrupts multi-line SQL containing commas (column lists, etc.).
  values = [
    yamlencode({
      primary = {
        initdb = {
          scripts = {
            "init.sql" = file("${path.module}/init.sql")
          }
        }
      }
    })
  ]

  set {
    name  = "metrics.enabled"
    value = "true"
  }

  set {
    name  = "metrics.serviceMonitor.enabled"
    value = "true"
  }

  set {
    name  = "primary.resources.requests.cpu"
    value = "100m"
  }

  set {
    name  = "primary.resources.requests.memory"
    value = "128Mi"
  }

  set {
    name  = "primary.resources.limits.cpu"
    value = "250m"
  }

  set {
    name  = "primary.resources.limits.memory"
    value = "256Mi"
  }
}

output "postgres_service_name" {
  value = "postgres-postgresql"
}
