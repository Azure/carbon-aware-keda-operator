provider "azurerm" {
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
}

resource "random_pet" "demo" {
  separator = ""
  length    = 2
}

resource "azurerm_resource_group" "demo" {
  name     = "rg-${random_pet.demo.id}"
  location = "westeurope"
}

resource "azurerm_log_analytics_workspace" "demo" {
  name                = "law-${random_pet.demo.id}"
  resource_group_name = azurerm_resource_group.demo.name
  location            = azurerm_resource_group.demo.location
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_kubernetes_cluster" "demo" {
  name                = "aks-${random_pet.demo.id}"
  resource_group_name = azurerm_resource_group.demo.name
  location            = azurerm_resource_group.demo.location
  dns_prefix          = "aks-${random_pet.demo.id}"

  default_node_pool {
    name                = "default"
    vm_size             = "Standard_D2s_v5"
    enable_auto_scaling = true
    min_count           = 1
    max_count           = 10
  }

  oms_agent {
    log_analytics_workspace_id = azurerm_log_analytics_workspace.demo.id
  }

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_redis_cache" "demo" {
  name                = "redis-${random_pet.demo.id}"
  location            = azurerm_resource_group.demo.location
  resource_group_name = azurerm_resource_group.demo.name
  capacity            = 0
  family              = "C"
  sku_name            = "Basic"
  enable_non_ssl_port = true
  minimum_tls_version = "1.2"

  redis_configuration {
  }
}

output "rg_name" {
  value = azurerm_resource_group.demo.name
}

output "rg_location" {
  value = azurerm_resource_group.demo.location
}

output "aks_name" {
  value = azurerm_kubernetes_cluster.demo.name
}

output "redis_password" {
  value     = azurerm_redis_cache.demo.primary_access_key
  sensitive = true
}

output "redis_hostname" {
  value = azurerm_redis_cache.demo.hostname
}
