rovider "aws" {
  region = var.region
  endpoints {
    eks = var.beta ? var.beta_endpoint : null
  }
}