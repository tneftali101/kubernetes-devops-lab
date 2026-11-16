
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 3.4"
    }
  }
}

provider "kubernetes" {
  config_path = pathexpand("~/.kube/config")
}

resource "kubernetes_namespace_v1" "creator_studio" {
  metadata {
    name = "creator-studio-dev"

    labels = {
      project     = "creator-studio"
      environment = "development"
      managed-by  = "terraform"
    }
  }
}

resource "kubernetes_resource_quota_v1" "creator_studio_quota" {
  metadata {
    name      = "creator-studio-quota"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name
  }

  spec {
    hard = {
      pods       = "20"
      services   = "10"
      configmaps = "20"
    }
  }
}

resource "kubernetes_config_map_v1" "creator_studio_config" {
  metadata {
    name      = "creator-studio-config"
    namespace = kubernetes_namespace_v1.creator_studio.metadata[0].name

    labels = {
      app        = "creator-studio"
      managed-by = "terraform"
    }
  }

  data = {
    APP_NAME      = "Creator Studio"
    APP_ENV       = "development"
    LOG_LEVEL     = "debug"
    SERVER_PORT   = "8080"
    DATABASE_HOST = "postgres"
    DATABASE_PORT = "5432"
  }
}
