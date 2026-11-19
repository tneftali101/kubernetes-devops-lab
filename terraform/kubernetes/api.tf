
# Creator Studio Go API Deployment

resource "kubernetes_deployment_v1" "creator_studio_api" {
  metadata {
    name      = "creator-studio-api"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name

    labels = {
      app = "creator-studio-api"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "creator-studio-api"
      }
    }

    template {
      metadata {
        labels = {
          app = "creator-studio-api"
        }
      }

      spec {
        container {
          name = "api"

          image             = "docker.io/library/creator-studio-api:v1"
          image_pull_policy = "Never"

          port {
            container_port = 8080
          }

          # Load application settings from our ConfigMap
          env_from {
            config_map_ref {
              name = kubernetes_config_map_v1.creator_studio_config.metadata[0].name
            }
          }

          # Load PostgreSQL password from our Secret
          env {
            name = "PGPASSWORD"

            value_from {
              secret_key_ref {
                name = "postgres-auth"
                key  = "POSTGRES_PASSWORD"
              }
            }
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 8080
            }

            initial_delay_seconds = 5
            period_seconds        = 10
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8080
            }

            initial_delay_seconds = 10
            period_seconds        = 20
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "64Mi"
            }

            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }
        }
      }
    }
  }
}


# Internal Kubernetes Service for the Go API

resource "kubernetes_service_v1" "creator_studio_api" {
  metadata {
    name      = "creator-studio-api"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name
  }

  spec {
    selector = {
      app = "creator-studio-api"
    }

    port {
      port        = 8080
      target_port = 8080
    }

    type = "ClusterIP"
  }
}
