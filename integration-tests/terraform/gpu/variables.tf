// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "k8s_version" {
  type    = string
  default = "1.30"
}

variable "addon_name" {
  type    = string
  default = "amazon-cloudwatch-observability"
}

variable "addon_version" {
  type    = string
  default = "v1.1.0-eksbuild.1"
}

variable "beta" {
  type    = bool
  default = false
}

variable "beta_endpoint" {
  type    = string
  default = ""
}

variable "eks_endpoint" {
  type    = string
  default = ""

  validation {
    condition     = can(regex("^[a-zA-Z0-9._:/-]*$", var.eks_endpoint))
    error_message = "eks_endpoint may only contain endpoint URL characters."
  }
}

variable "dcgm_image" {
  type = object({
    registry   = string
    repository = string
    tag        = string
  })
  default = {
    registry   = "nvcr.io/nvidia/k8s"
    repository = "dcgm-exporter"
    tag        = ""
  }

  validation {
    condition = (
      can(regex("^[a-zA-Z0-9._/:-]*$", var.dcgm_image.registry)) &&
      can(regex("^[a-zA-Z0-9._/-]*$", var.dcgm_image.repository)) &&
      can(regex("^[a-zA-Z0-9._-]*$", var.dcgm_image.tag))
    )
    error_message = "dcgm_image fields may only contain image-reference characters."
  }
}
