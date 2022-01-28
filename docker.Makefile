include common.mk

export BUILD_BASE_IMAGE = golang:1.16.13-alpine3.14
export TEST_BASE_IMAGE = golang:1.16.13
export RUN_BASE_IMAGE = alpine:3.14
export KUBERNETES_VERSION ?= 1.16.1
KIND_VERSION ?= 0.5.1
IMAGES = ${IMAGE_REPO_PREFIX}controller ${IMAGE_REPO_PREFIX}controller-coverage ${IMAGE_REPO_PREFIX}e2e-tests ${IMAGE_REPO_PREFIX}e2e-benchmark ${IMAGE_REPO_PREFIX}api-server ${IMAGE_REPO_PREFIX}api-server-coverage ${IMAGE_REPO_PREFIX}installer
PUSH_IMAGES = push/${IMAGE_REPO_PREFIX}controller push/${IMAGE_REPO_PREFIX}api-server push/${IMAGE_REPO_PREFIX}installer
DOCKERFILE = Dockerfile
ifneq ($(DOCKER_BUILDKIT),)
  override DOCKERFILE := Dockerfile.buildkit
endif

BUILD_ARGS = \
  --build-arg BUILD_BASE=${BUILD_BASE_IMAGE} \
  --build-arg RUN_BASE=${RUN_BASE_IMAGE} \
  --build-arg BUILDTIME \
  --build-arg GITCOMMIT \
  --build-arg VERSION \
  --build-arg KUBERNETES_VERSION \
  --build-arg IMAGE_REPO_PREFIX \
  --build-arg GOPROXY

build-validate-image: ## create the image used for validation
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e dockerfiles scripts gometalinter.json Gopkg.toml Gopkg.lock .git | docker build --build-arg BUILD_BASE=${BUILD_BASE_IMAGE} -t kube-compose-validate -f ./dockerfiles/Dockerfile.dev --target lint -

build-vendor-image:
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal e2e dockerfiles scripts gometalinter.json Gopkg.toml Gopkg.lock .git | docker build --build-arg BUILD_BASE=${BUILD_BASE_IMAGE} -t kube-compose-vendor -f ./dockerfiles/Dockerfile.dev --target dev -

validate: validate-lint validate-pkg validate-vendor ## validate lint and package dependencies

validate-lint: build-validate-image ## linter
	docker run -ti --rm kube-compose-validate

validate-pkg: build-validate-image ## validate that no public package used in the CLI depends on internal stuff
	docker run -ti --rm --entrypoint=/bin/bash kube-compose-validate ./scripts/validate-pkg

validate-vendor: build-vendor-image
	docker run -ti --rm -v kube-compose-vendor-cache://dep-cache -e DEPCACHEDIR=//dep-cache kube-compose-vendor make validate-vendor

vendor: build-vendor-image
	$(info Update Vendoring...)
	docker rm -f kube-compose-vendoring || true
	# git bash, mingw and msys by default rewrite args that seems to be linux paths and try to expand that to a meaningful windows path
	# we don't want that to happen when mounting paths referring to files located in the container. Thus we use the double "//" prefix that works
	# both on windows, linux and macos
	docker run -it --name kube-compose-vendoring -v kube-compose-vendor-cache://dep-cache -e DEPCACHEDIR=//dep-cache kube-compose-vendor sh -c "make vendor DEP_ARGS=\"$(DEP_ARGS)\""
	rm -rf ./vendor
	docker cp kube-compose-vendoring:/go/src/github.com/docker/compose-on-kubernetes/vendor .
	docker cp kube-compose-vendoring:/go/src/github.com/docker/compose-on-kubernetes/Gopkg.lock .
	docker rm -f kube-compose-vendoring
	$(warning You may need to reset permissions on vendor/*)

${IMAGE_REPO_PREFIX}e2e-tests:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}e2e-tests:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}e2e-tests:$(TAG) --target compose-e2e-tests -f ${DOCKERFILE}  -

${IMAGE_REPO_PREFIX}e2e-benchmark:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}e2e-benchmark:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}e2e-benchmark:$(TAG) --target compose-e2e-benchmark -f ${DOCKERFILE}  -

${IMAGE_REPO_PREFIX}api-server:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}api-server:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}api-server:$(TAG) --target compose-api-server -f ${DOCKERFILE} -

${IMAGE_REPO_PREFIX}api-server-coverage:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}api-server-coverage:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}api-server-coverage:$(TAG) --target compose-api-server-coverage -f ${DOCKERFILE} -

${IMAGE_REPO_PREFIX}controller:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}controller:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}controller:$(TAG) -f ${DOCKERFILE} -

${IMAGE_REPO_PREFIX}controller-coverage:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}controller-coverage:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}controller-coverage:$(TAG) --target compose-controller-coverage -f ${DOCKERFILE} -

${IMAGE_REPO_PREFIX}installer:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}installer:${TAG}"
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e | docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}installer:$(TAG) --target compose-installer -f ${DOCKERFILE}  -

images: $(IMAGES) ## build images
	@echo $(IMAGES)

save-coverage-images:
	@echo "🌟 save ${IMAGE_REPO_PREFIX}api-server-coverage:$(TAG) ${IMAGE_REPO_PREFIX}controller-coverage:$(TAG)"
	@mkdir -p pre-loaded-images
	@docker save -o pre-loaded-images/coverage-enabled-images.tar ${IMAGE_REPO_PREFIX}api-server-coverage:$(TAG) ${IMAGE_REPO_PREFIX}controller-coverage:$(TAG)

install-kind:
ifeq (,$(wildcard ./~/bin/kind))
	curl -fLo ~/bin/kind --create-dirs https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-$$(uname)-amd64
	chmod +x ~/bin/kind
endif

create-kind-cluster: install-kind
	@echo "🌟 Create kind cluster"
	@kind create cluster --name compose-on-kube --image kindest/node:v$(KUBERNETES_VERSION)

load-kind-image-archive:
	@echo "🌟 Load archive $(IMAGE_ARCHIVE) in kind cluster"
	@kind load image-archive --name compose-on-kube $(IMAGE_ARCHIVE)

delete-kind-cluster:
	@echo "🌟 Delete kind cluster"
	kind delete cluster --name compose-on-kube

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
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

test-unit:
	@tar cf - ${DOCKERFILE} doc.go Makefile common.mk cmd install api internal vendor e2e dockerfiles/Dockerfile.test | docker build -t docker/kube-compose-unit-test-base --build-arg TEST_BASE=$(TEST_BASE_IMAGE) -f dockerfiles/Dockerfile.test -
	docker rm -f kube-compose-unit-test || exit 0
	docker create --name kube-compose-unit-test docker/kube-compose-unit-test-base
	@docker cp kube-compose-unit-test:/go/src/github.com/docker/compose-on-kubernetes/coverage.txt ./coverage.txt
	docker rm -f kube-compose-unit-test
