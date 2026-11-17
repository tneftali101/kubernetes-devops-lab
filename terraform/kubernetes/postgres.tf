
resource "kubernetes_service_v1" "postgres" {
  metadata {
    name      = "postgres"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name
  }

  spec {
    selector = {
      app = "postgres"
    }

    port {
      name        = "postgres"
      port        = 5432
      target_port = 5432
    }

    cluster_ip = "None"
  }
}

resource "kubernetes_stateful_set_v1" "postgres" {
  metadata {
    name      = "postgres"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name
  }

  spec {
    service_name = kubernetes_service_v1.postgres.metadata[0].name
    replicas     = 1

    selector {
      match_labels = {
        app = "postgres"
      }
    }

    template {
      metadata {
        labels = {
          app = "postgres"
        }
      }

      spec {
        container {
          name  = "postgres"
          image = "postgres:17"

          port {
            container_port = 5432
            name           = "postgres"
          }

          env {
            name  = "POSTGRES_DB"
            value = "creator_studio"
          }

          env {
            name  = "POSTGRES_USER"
            value = "creator_studio"
          }

          env {
            name = "POSTGRES_PASSWORD"

            value_from {
              secret_key_ref {
                name = "postgres-auth"
                key  = "POSTGRES_PASSWORD"
              }
            }
          }

          env {
            name  = "PGDATA"
            value = "/var/lib/postgresql/data/pgdata"
          }

          volume_mount {
            name       = "postgres-data"
            mount_path = "/var/lib/postgresql/data"
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "256Mi"
            }

            limits = {
              cpu    = "500m"
              memory = "1Gi"
            }
          }

          readiness_probe {
            exec {
              command = [
                "sh",
                "-c",
                "pg_isready -U creator_studio -d creator_studio"
              ]
            }

            initial_delay_seconds = 10
            period_seconds        = 10
          }

          liveness_probe {
            exec {
              command = [
                "sh",
                "-c",
                "pg_isready -U creator_studio -d creator_studio"
              ]
            }

            initial_delay_seconds = 30
            period_seconds        = 20
          }
        }
      }
    }

    volume_claim_template {
      metadata {
        name = "postgres-data"
      }

      spec {
        access_modes       = ["ReadWriteOnce"]
        storage_class_name = "local-path"

        resources {
          requests = {
            storage = "5Gi"
          }
        }
      }
    }
  }
}
