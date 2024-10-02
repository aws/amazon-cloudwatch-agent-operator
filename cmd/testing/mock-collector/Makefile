ARCH ?= "amd64" #$(shell go env GOARCH)
IMG ?= "956457624121.dkr.ecr.us-west-2.amazonaws.com/agent-debug-cont:latest"
.PHONY: container
container:
	docker buildx build --load --platform linux/${ARCH} -t ${IMG} .

# Push the container image, used only for local dev purposes
.PHONY: container-push
container-push:
	docker push ${IMG}

.PHONY: build-and-push
build-and-push: container
build-and-push: container-push
