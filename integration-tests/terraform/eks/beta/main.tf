// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

module "common" {
  source             = "../../common"
}

module "basic_components" {
  source = "../../basic_components"
}

resource "aws_cloudformation_stack" "service_role" {
  name = "cwagent-operator-eks-beta-service-role"
  capabilities = ["CAPABILITY_IAM"]
  template_body = file("${path.module}/amazon-eks-service-role.yaml")
}

resource "aws_cloudformation_stack" "vpc_stack" {
  name = "cwagent-operator-eks-beta-vpc-stack"
  capabilities = ["CAPABILITY_IAM"]
  template_body = file("${path.module}/amazon-eks-vpc-sample.yaml")
}

resource "aws_eks_cluster" "this" {
  name     = "cwagent-operator-eks-beta-integ-${module.common.testing_id}"
  role_arn = aws_cloudformation_stack.service_role.outputs.RoleArn
  version  = var.k8s_version
  vpc_config {
    subnet_ids         = split(",",aws_cloudformation_stack.vpc_stack.outputs.SubnetIds)
    security_group_ids = split(",",aws_cloudformation_stack.vpc_stack.outputs.SecurityGroups)
  }
  depends_on = [
    aws_cloudformation_stack.vpc_stack
  ]
}

resource "null_resource" "kubectl" {
  depends_on = [
    aws_eks_cluster.this
  ]
  provisioner "local-exec" {
    command = "aws eks --endpoint ${var.beta_endpoint} --region ${var.region} update-kubeconfig --name ${aws_eks_cluster.this.name}"
    command = <<-EOT
      aws eks --endpoint ${var.beta_endpoint} --region ${var.region} update-kubeconfig --name ${aws_eks_cluster.this.name}
      aws eks --endpoint ${var.beta_endpoint} --region ${var.region} list-clusters --output text
      aws eks --endpoint ${var.beta_endpoint} --region ${var.region} describe-cluster --name ${aws_eks_cluster.this.name} --output text
    EOT
  }
}

resource "aws_cloudformation_stack" "node-stack" {
  name = "${aws_eks_cluster.this.name}-nodegroup"
  capabilities = ["CAPABILITY_IAM"]
  template_body = file("${path.module}/amazon-eks-nodegroup.yaml")
  parameters = {
    NodeInstanceType="t3.medium"
    NodeAutoScalingGroupMinSize=1
    NodeAutoScalingGroupMaxSize=1
    NodeAutoScalingGroupDesiredCapacity=1
    NodeImageId="ami-015a336f2a25fc752"
    ClusterName=aws_eks_cluster.this.name
    NodeGroupName="${aws_eks_cluster.this.name}-nodegroup"
    ClusterControlPlaneSecurityGroup=aws_cloudformation_stack.vpc_stack.outputs.SecurityGroups
    VpcId=aws_cloudformation_stack.vpc_stack.outputs.VpcId
    DisableIMDSv1=true
    Subnets=aws_cloudformation_stack.vpc_stack.outputs.SubnetIds
  }
  depends_on = [
    aws_eks_cluster.this
  ]
}

resource "null_resource" "apply_auth" {
  depends_on = [
    aws_cloudformation_stack.node-stack,
    null_resource.kubectl
  ]
  provisioner "local-exec" {
    command = "auth_config.sh"
    interpreter = ["/bin/bash"]
    working_dir = path.module
    environment = {
      NODE_ROLE=aws_cloudformation_stack.node-stack.outputs.NodeInstanceRole
      CLUSTER_ARN=aws_eks_cluster.this.arn
    }
  }
}

resource "null_resource" "eks-addon" {
  depends_on = [
    aws_cloudformation_stack.node-stack,
    null_resource.apply_auth
  ]
  provisioner "local-exec" {
    command = <<-EOT
      echo "Adding EKS addon"
      aws eks --endpoint ${var.beta_endpoint} --region ${var.region} create-addon --cluster-name ${aws_eks_cluster.this.name} --addon-name ${var.addon}
      sleep 100
    EOT
  }
}

resource "null_resource" "test-resources" {
  depends_on = [
    null_resource.eks-addon
  ]
  provisioner "local-exec" {
    command = <<-EOT
      kubectl get pods -A
      kubectl get services --namespace amazon-cloudwatch
      kubectl get all --all-namespaces
    EOT
  }
}

resource "null_resource" "validator" {
  depends_on = [
    null_resource.eks-addon
  ]
  provisioner "local-exec" {
    command = "go test ${var.test_dir} -v"
  }
}