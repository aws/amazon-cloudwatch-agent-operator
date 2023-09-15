// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "k8s_version" {
  type    = string
  default = "1.24"
}

variable "test_dir" {
  type    = string
  default = "../../../eks-addon"
}

variable "addon" {
  type = string
  default = "amazon-cloudwatch"
}

variable "beta_endpoint" {
  type = string
  default = "https://api.beta.us-west-2.wesley.amazonaws.com"
}