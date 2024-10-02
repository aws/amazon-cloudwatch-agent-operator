aws_account_id="956457624121"
repository="prom-sample-app"
region="us-west-2"

aws ecr get-login-password --region $region | docker login --username AWS --password-stdin $aws_account_id.dkr.ecr.$region.amazonaws.com

docker build --platform=linux/amd64 -t prometheus_sample_app .
docker tag prometheus_sample_app:latest $aws_account_id.dkr.ecr.$region.amazonaws.com/$repository:latest
docker push $aws_account_id.dkr.ecr.$region.amazonaws.com/$repository:latest


NEW_IMAGE=$aws_account_id.dkr.ecr.$region.amazonaws.com/$repository:latest
sed -i '' "s|\$IMAGE|$NEW_IMAGE|g" prometheus-sample-app-k8s-deployment.yaml

kubectl apply -f prometheus-sample-app-k8s-deployment.yaml

