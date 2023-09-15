// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

module "common" {
  source             = "../../common"
}

module "basic_components" {
  source = "../../basic_components"

  region = var.region
}

data "aws_eks_cluster_auth" "this" {
  name = aws_eks_cluster.this.name
}

resource "aws_eks_cluster" "this" {
  name     = "cwagent-operator-eks-beta-integ-${module.common.testing_id}"
  role_arn = module.basic_components.role_arn
  version  = var.k8s_version
  vpc_config {
    subnet_ids         = module.basic_components.public_subnet_ids
    security_group_ids = [module.basic_components.security_group]
  }
}


resource "null_resource" "kubectl" {
  depends_on = [
    aws_eks_cluster.this
  ]
  provisioner "local-exec" {
    command = "aws eks --endpoint ${var.beta_endpoint} --region ${var.region} update-kubeconfig --name ${aws_eks_cluster.this.name}"
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
    ClusterControlPlaneSecurityGroup=module.basic_components.security_group
    VpcId=module.basic_components.vpc_id
    DisableIMDSv1=true
    Subnets=join(", ", module.basic_components.public_subnet_ids)
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

resource "null_resource" "validator" {
  depends_on = [
    null_resource.eks-addon
  ]
  provisioner "local-exec" {
    command = "go test ${var.test_dir} -v"
  }
}