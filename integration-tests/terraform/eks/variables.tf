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
  default = ""
}

variable "addon" {
  type = string
  default = "amazon-cloudwatch"
}

variable "typeOfTest" {
  type        = string
  default     = "operator"
  description = "Defaults to operator. Possible options are operator/add-on. this is for conditionally creating resources"
  # see https://stackoverflow.com/questions/72087946/how-to-conditional-create-resource-in-terraform-based-on-a-string-variable
}