include docker.Makefile
include Makefile

DEBUGABLE_IMAGES = ${IMAGE_REPO_PREFIX}controller ${IMAGE_REPO_PREFIX}api-server

${IMAGE_REPO_PREFIX}api-server-debug:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}api-server-debug:${TAG}"
	@DOCKER_BUILDKIT=true docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}api-server-debug:$(TAG) --target compose-api-server-${LIVE_DEBUG}debug -f ./Dockerfile.debuggable .

${IMAGE_REPO_PREFIX}controller-debug:
	@echo "🌟 build ${IMAGE_REPO_PREFIX}controller-debug:${TAG}"
	@DOCKER_BUILDKIT=true docker build ${BUILD_ARGS} -t ${IMAGE_REPO_PREFIX}controller-debug:$(TAG) --target compose-controller-${LIVE_DEBUG}debug -f ./Dockerfile.debuggable .

debug-images: $(addsuffix -debug,$(DEBUGABLE_IMAGES))
	@echo $(addsuffix -debug,$(DEBUGABLE_IMAGES))

install-live-debug-images: LIVE_DEBUG=live-
install-live-debug-images: debug-images bin/debug-installer
	@bin/debug-installer$(EXEC_EXT)

install-debug-images: LIVE_DEBUG=
install-debug-images: debug-images bin/debug-installer
	@bin/debug-installer$(EXEC_EXT)