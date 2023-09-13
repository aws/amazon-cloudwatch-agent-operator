// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

output "testing_id" {
  value = random_id.testing_id.hex
}

output "cwa_iam_role" {
  value = "cwa-e2e-iam-role"
}

output "cwa_iam_policy" {
  value = "cwa-e2e-iam-policy"
}

output "cwa_iam_instance_profile" {
  value = "cwa-e2e-iam-instance-profile"
}

output "vpc_security_group" {
  value = "vpc_security_group"
}

