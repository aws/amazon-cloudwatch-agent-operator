// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "test_dir" {
  type    = string
  default = "../../gpu"
}

variable "addon_name" {
  type    = string
  default = "amazon-cloudwatch-observability"
}

variable "addon_version" {
  type = string
  default = "v1.6.0-eksbuild.1"
}


variable "cwagent_image_repo" {
  type    = string
  default = "public.ecr.aws/cloudwatch-agent/cloudwatch-agent"
}

variable "cwagent_image_tag" {
  type    = string
  default = "latest"
}

variable "k8s_version" {
  type    = string
  default = "1.28"
}

variable "ami_type" {
  type    = string
  default = "AL2_x86_64"
}

variable "instance_type" {
  type    = string
  default = "t3.medium"
}

variable "beta" {
  type    = bool
  default = false
}

variable "beta_endpoint" {
  type    = string
  default = "https://api.beta.us-west-2.wesley.amazonaws.com"
}