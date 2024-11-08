make container
# docker tag docker.io/aws/cloudwatch-agent-operator:tls  956457624121.dkr.ecr.us-west-2.amazonaws.com/operator:tls 
docker tag docker.io/aws/cloudwatch-agent-operator:tls  956457624121.dkr.ecr.us-west-2.amazonaws.com/operator:latest 
# docker push 956457624121.dkr.ecr.us-west-2.amazonaws.com/operator:tls
docker push 956457624121.dkr.ecr.us-west-2.amazonaws.com/operator:latest