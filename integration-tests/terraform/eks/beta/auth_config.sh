#!/usr/bin/env bash

S3_URL="https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2020-10-29"

AUTHENTICATOR_CM_YAML="aws-auth-cm.yaml"
curl -s -O $S3_URL/$AUTHENTICATOR_CM_YAML
echo "Applying Authenticator ConfigMap with $NODE_ROLE and $CLUSTER_ARN"
cat aws-auth-cm.yaml | sed -e "s|rolearn.*$|rolearn: $NODE_ROLE|" | kubectl --context $CLUSTER_ARN apply -f -
rm -f $AUTHENTICATOR_CM_YAML

sleep 120