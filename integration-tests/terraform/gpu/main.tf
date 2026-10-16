// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

module "common" {
  source = "../common"
}

module "basic_components" {
  source = "../basic_components"
}

locals {
  eks_endpoint = var.eks_endpoint != "" ? var.eks_endpoint : (var.beta ? var.beta_endpoint : "")
  role_arn     = format("%s%s", module.basic_components.role_arn, var.beta ? "-eks-beta" : "")
  aws_eks      = format("%s%s", "aws eks --region ${var.region}", local.eks_endpoint != "" ? " --endpoint ${local.eks_endpoint}" : "")

  dcgm_image_ref = "${var.dcgm_image.registry}/${var.dcgm_image.repository}:${var.dcgm_image.tag}"
}

data "aws_eks_cluster_auth" "this" {
  name = aws_eks_cluster.this.name
}

resource "aws_eks_cluster" "this" {
  name     = "cwagent-operator-eks-integ-${module.common.testing_id}"
  role_arn = local.role_arn
  version  = var.k8s_version
  vpc_config {
    subnet_ids         = module.basic_components.public_subnet_ids
    security_group_ids = [module.basic_components.security_group]
  }
}

# EKS Node Groups
resource "aws_eks_node_group" "this" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "cwagent-operator-eks-integ-node"
  node_role_arn   = aws_iam_role.node_role.arn
  subnet_ids      = module.basic_components.public_subnet_ids

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  ami_type       = "AL2023_x86_64_NVIDIA"
  capacity_type  = "ON_DEMAND"
  disk_size      = 40
  instance_types = ["g4dn.xlarge"]

  depends_on = [
    aws_iam_role_policy_attachment.node_AmazonEC2ContainerRegistryReadOnly,
    aws_iam_role_policy_attachment.node_AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node_AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node_CloudWatchAgentServerPolicy
  ]
}

# EKS Node IAM Role
resource "aws_iam_role" "node_role" {
  name = "cwagent-operator-eks-Worker-Role-${module.common.testing_id}"

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
POLICY
}

resource "aws_iam_role_policy_attachment" "node_AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node_role.name
}

resource "aws_iam_role_policy_attachment" "node_AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.node_role.name
}

resource "aws_iam_role_policy_attachment" "node_AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.node_role.name
}

resource "aws_iam_role_policy_attachment" "node_CloudWatchAgentServerPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
  role       = aws_iam_role.node_role.name
}

resource "null_resource" "kubectl" {
  depends_on = [
    aws_eks_cluster.this,
    aws_eks_node_group.this
  ]
  provisioner "local-exec" {
    command = <<-EOT
      ${local.aws_eks} update-kubeconfig --name ${aws_eks_cluster.this.name}
      ${local.aws_eks} list-clusters --output text
      ${local.aws_eks} describe-cluster --name ${aws_eks_cluster.this.name} --output text
    EOT
  }
}

resource "null_resource" "nvidia_device_plugin" {
  depends_on = [
    null_resource.kubectl
  ]
  provisioner "local-exec" {
    command = "kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.0/deployments/static/nvidia-device-plugin.yml"
  }
}

resource "aws_eks_addon" "this" {
  depends_on = [
    null_resource.kubectl,
    null_resource.nvidia_device_plugin
  ]
  addon_name    = var.addon_name
  cluster_name  = aws_eks_cluster.this.name
  addon_version = var.addon_version
}

resource "null_resource" "dcgm_image_override" {
  count = var.dcgm_image.tag == "" ? 0 : 1
  depends_on = [
    aws_eks_addon.this
  ]
  provisioner "local-exec" {
    environment = {
      DCGM_IMAGE = local.dcgm_image_ref
    }
    command = <<-EOT
      set -eu
      name=$(kubectl get dcgmexporter -n amazon-cloudwatch -o name | head -n1)
      [ -n "$name" ] || { echo "no DcgmExporter CR found" >&2; exit 1; }
      kubectl patch "$name" -n amazon-cloudwatch --type=merge -p "{\"spec\":{\"image\":\"$DCGM_IMAGE\"}}"
      kubectl rollout status "daemonset/$${name#*/}" -n amazon-cloudwatch --timeout=300s
      kubectl get "daemonset/$${name#*/}" -n amazon-cloudwatch -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -qxF "$DCGM_IMAGE"
    EOT
  }
}

resource "null_resource" "validator" {
  depends_on = [
    aws_eks_addon.this,
    null_resource.dcgm_image_override
  ]
  provisioner "local-exec" {
    command = <<-EOT
      set -eu
      name=$(kubectl get dcgmexporter -n amazon-cloudwatch -o name | head -n1)
      [ -n "$name" ] || { echo "no DcgmExporter CR found" >&2; exit 1; }
      ds="$${name#*/}"
      kubectl rollout status "daemonset/$ds" -n amazon-cloudwatch --timeout=300s
      pod=$(kubectl get pods -n amazon-cloudwatch -o json | jq -r --arg ds "$ds" '.items[] | select(any(.metadata.ownerReferences[]?; .name == $ds)) | .metadata.name' | head -n1)
      [ -n "$pod" ] || { echo "no dcgm pod found for daemonset $ds" >&2; exit 1; }
      kubectl wait --for=condition=Ready "pod/$pod" -n amazon-cloudwatch --timeout=300s
      kubectl get --raw "/api/v1/namespaces/amazon-cloudwatch/pods/$pod:9400/proxy/metrics" | grep -q "DCGM_FI_"
    EOT
  }
}
