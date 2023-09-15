#!/usr/bin/env bash

mkdir ${{ github.workspace }}/dist && make kustomize && make release-artifacts IMG=506463145083.dkr.ecr.us-west-2.amazonaws.com/cwagent-operator-pre-release:latest