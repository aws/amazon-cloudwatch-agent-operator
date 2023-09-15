// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

provider "aws" {
  region = var.region
  endpoints {
    eks = var.beta_endpoint
  }
}