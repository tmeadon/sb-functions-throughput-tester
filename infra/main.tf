provider "azurerm" {
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
}

locals {
  tags = {
    scenario = "sb-functions-throughput-test"
  }
  unique_name = replace(replace(lower(random_id.random.b64_url), "-", ""), "_", "")
}


resource "random_id" "random" {
  keepers = {
    "rg_name" = azurerm_resource_group.rg.name
  }
  byte_length = 8
}

resource "azurerm_resource_group" "rg" {
  name     = var.base_name
  location = var.location
}

resource "azurerm_servicebus_namespace" "sb_ns" {
  name                = "sb${local.unique_name}"
  location            = var.location
  resource_group_name = azurerm_resource_group.rg.name

  sku      = "Premium"
  capacity = 2
}

resource "azurerm_servicebus_queue" "sb_in" {
  name = "in"

  namespace_id        = azurerm_servicebus_namespace.sb_ns.id
  enable_partitioning = false
}

resource "azurerm_servicebus_queue" "sb_out" {
  name = "out"

  namespace_id        = azurerm_servicebus_namespace.sb_ns.id
  enable_partitioning = false
}

resource "azurerm_storage_account" "stg" {
  name                = local.unique_name
  location            = var.location
  resource_group_name = azurerm_resource_group.rg.name

  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_service_plan" "asp" {
  name                = var.base_name
  location            = var.location
  resource_group_name = azurerm_resource_group.rg.name

  os_type      = "Windows"
  sku_name     = "S1"
  worker_count = 2
}

resource "azurerm_application_insights" "ai" {
  name                = var.base_name
  location            = var.location
  resource_group_name = azurerm_resource_group.rg.name

  application_type = "web"
}

resource "azurerm_windows_function_app" "func" {
  name                = var.base_name
  location            = var.location
  resource_group_name = azurerm_resource_group.rg.name

  storage_account_name       = azurerm_storage_account.stg.name
  storage_account_access_key = azurerm_storage_account.stg.primary_access_key
  service_plan_id            = azurerm_service_plan.asp.id

  site_config {
    application_insights_key               = azurerm_application_insights.ai.instrumentation_key
    application_insights_connection_string = azurerm_application_insights.ai.connection_string
    always_on                              = true

    application_stack {
      dotnet_version              = "6"
      use_dotnet_isolated_runtime = false
    }
  }

  connection_string {
    name  = "sb_conn"
    type  = "Custom"
    value = azurerm_servicebus_namespace.sb_ns.default_primary_connection_string
  }

  app_settings = {
    # "sb_conn" = azurerm_servicebus_namespace.sb_ns.default_primary_connection_string
    "WEBSITE_RUN_FROM_PACKAGE" = "1"
  }
}

output "function_app_name" {
  value = azurerm_windows_function_app.func.name
}

output "resource_group_name" {
  value = azurerm_resource_group.rg.name
}

output "service_bus_connection" {
  value     = azurerm_servicebus_namespace.sb_ns.default_primary_connection_string
  sensitive = true
}
