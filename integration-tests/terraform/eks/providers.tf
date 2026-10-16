// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

provider "aws" {
  region = var.region
  endpoints {
    eks = local.eks_endpoint != "" ? local.eks_endpoint : null
  }
}
