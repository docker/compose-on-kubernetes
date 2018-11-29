include common.mk

BUILD_BASE_IMAGE = golang:1.11.1-alpine3.8
TEST_BASE_IMAGE = golang:1.11.1
RUN_BASE_IMAGE = alpine:3.8
IMAGES = ${IMAGE_REPO_PREFIX}controller ${IMAGE_REPO_PREFIX}controller-coverage ${IMAGE_REPO_PREFIX}e2e-tests ${IMAGE_REPO_PREFIX}api-server ${IMAGE_REPO_PREFIX}api-server-coverage ${IMAGE_REPO_PREFIX}installer ${IMAGE_REPO_PREFIX}updater
PUSH_IMAGES = push/${IMAGE_REPO_PREFIX}controller push/${IMAGE_REPO_PREFIX}api-server push/${IMAGE_REPO_PREFIX}installer push/${IMAGE_REPO_PREFIX}updater

BUILD_ARGS = \
  --build-arg BUILD_BASE=${BUILD_BASE_IMAGE} \
  --build-arg RUN_BASE=${RUN_BASE_IMAGE} \
  --build-arg BUILDTIME=${BUILDTIME} \
  --build-arg GITCOMMIT=${GITCOMMIT} \
  --build-arg VERSION=${VERSION} \
  --build-arg IMAGE_REPO_PREFIX=${IMAGE_REPO_PREFIX}

build-validate-image: ## create the image used for validation
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e dockerfiles scripts gometalinter.json | docker build --build-arg BUILD_BASE=${BUILD_BASE_IMAGE} -t kube-compose-validate -f ./dockerfiles/Dockerfile.lint -

validate: validate-lint validate-pkg ## validate lint and package dependencies

validate-lint: build-validate-image ## linter
	docker run -ti kube-compose-validate

validate-pkg: build-validate-image ## validate that no public package used in the CLI depends on internal stuff
	docker run -ti --entrypoint=/bin/bash kube-compose-validate ./scripts/validate-pkg

${IMAGE_REPO_PREFIX}e2e-tests:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}e2e-tests:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}e2e-tests:$(TAG) --target compose-e2e-tests  -

${IMAGE_REPO_PREFIX}api-server:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}api-server:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}api-server:$(TAG) --target compose-api-server  -

${IMAGE_REPO_PREFIX}api-server-coverage:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}api-server-coverage:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}api-server-coverage:$(TAG) --target compose-api-server-coverage -

${IMAGE_REPO_PREFIX}controller:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}controller:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}controller:$(TAG) -

${IMAGE_REPO_PREFIX}controller-coverage:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}controller-coverage:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}controller-coverage:$(TAG) --target compose-controller-coverage -

${IMAGE_REPO_PREFIX}installer:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}installer:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}installer:$(TAG) --target compose-installer  -

${IMAGE_REPO_PREFIX}updater:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}updater:${TAG}"
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}updater:$(TAG) --target compose-updater  -

images: $(IMAGES) ## build images
	@echo $(IMAGES)

push/%:
	@echo "🌟 push $@ ($*:$(TAG))"
	@docker push "$*:$(TAG)"

push: $(PUSH_IMAGES) ## push images
	@echo $(PUSH_IMAGES)

install: ${IMAGE_REPO_PREFIX}installer ## install compose kubernetes component
	@echo "🌟 installing compose"
	@docker run --net=host --rm -v ~/.kube/config:/root/.kube/config ${IMAGE_REPO_PREFIX}installer -etcd-servers http://127.0.0.1:2379 -network-host

uninstall: ${IMAGE_REPO_PREFIX}installer ## uninstall compose kubernetes component
	@echo "🌟 uninstalling compose"
	@docker run --net=host --rm -v ~/.kube/config:/root/.kube/config ${IMAGE_REPO_PREFIX}installer -uninstall

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(Makefile_LIST) | sort

test-unit:
	@tar cf - Dockerfile doc.go Makefile common.mk cmd install api internal vendor e2e dockerfiles/Dockerfile.test | docker build -t docker/kube-compose-unit-test-base --build-arg TEST_BASE=$(TEST_BASE_IMAGE) -f dockerfiles/Dockerfile.test -
	docker rm -f kube-compose-unit-test || exit 0
	docker create --name kube-compose-unit-test docker/kube-compose-unit-test-base
	@docker cp kube-compose-unit-test:/go/src/github.com/docker/compose-on-kubernetes/coverage.txt ./coverage.txt
	docker rm -f kube-compose-unit-test