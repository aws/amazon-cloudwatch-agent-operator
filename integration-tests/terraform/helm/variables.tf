// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "k8s_version" {
  type    = string
  default = "1.25"
}

# eks addon and helm tests are similar
variable "test_dir" {
  type    = string
  default = "../../eks-addon"
}

variable "helm_dir" {
  type    = string
  default = "../../../helm"
}

variable "kube_dir" {
  type    = string
  default = "~/.kube"
}

variable "cluster_name" {
  type    = string
  default = "cwagent-operator-helm-integ"
}

variable "cluster_namespace" {
  type = string
  default = "amazon-cloudwatch"
}
