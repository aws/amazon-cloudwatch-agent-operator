// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

module "common" {
  source             = "../common"
  cwagent_image_repo = var.cwagent_image_repo
  cwagent_image_tag  = var.cwagent_image_tag
}

module "basic_components" {
  source = "../basic_components"

  region = var.region
}


data "aws_eks_cluster_auth" "this" {
  name = aws_eks_cluster.this.name
}

locals {
  role_arn = format("%s%s", module.basic_components.role_arn, var.beta ? "-eks-beta" : "")
  aws_eks  = format("%s%s", "aws eks --region ${var.region}", var.beta ? " --endpoint ${var.beta_endpoint}" : "")
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

  ami_type       = "AL2_x86_64_GPU"
  capacity_type  = "ON_DEMAND"
  disk_size      = 20
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

resource "aws_eks_addon" "this" {
  depends_on = [
    null_resource.kubectl
  ]
  addon_name   = var.addon_name
  cluster_name = aws_eks_cluster.this.name
  addon_version = var.addon_version
}

resource "null_resource" "validator" {
  depends_on = [
      aws_eks_node_group.this,
      aws_eks_addon.this
  ]

  provisioner "local-exec" {
    command = <<EOT
      go test ${var.test_dir} -eksClusterName ${aws_eks_cluster.this.name} -computeType=EKS -v -eksDeploymentStrategy=DAEMON -eksGpuType=nvidia

      # Get all pods and describe them
      kubectl get pods --all-namespaces -o wide > pods.txt
      kubectl describe pods --all-namespaces > pods_describe.txt

      # Log the contents of the files
      cat pods.txt
      cat pods_describe.txt
    EOT
  }
}

