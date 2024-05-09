// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

output "vpc_id" {
  value = data.aws_vpc.vpc.id
}

output "security_group" {
  value = data.aws_security_group.security_group.id
}

output "public_subnet_ids" {
  value = data.aws_subnets.public_subnet_ids.ids
}

output "role_arn" {
  value = data.aws_iam_role.cwagent_iam_role.arn
}

output "instance_profile" {
  value = data.aws_iam_instance_profile.cwagent_instance_profile.name
}