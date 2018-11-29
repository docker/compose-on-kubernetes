ARG BUILD_BASE
ARG RUN_BASE

FROM ${RUN_BASE} AS runbase
RUN apk add ca-certificates --no-cache

FROM ${BUILD_BASE} AS build

RUN apk add --no-cache \
  coreutils \
  make

WORKDIR /go/src/github.com/docker/compose-on-kubernetes
COPY . .
ARG BUILDTIME
ARG GITCOMMIT
ARG VERSION
ARG IMAGE_REPO_PREFIX
ENV GITCOMMIT=$GITCOMMIT VERSION=$VERSION BUILDTIME=$BUILDTIME IMAGE_REPO_PREFIX=$IMAGE_REPO_PREFIX
ENV CGO_ENABLED=0
RUN make bin/compose-controller bin/compose-controller.test e2e-binary bin/installer bin/api-server bin/api-server.test

# e2e-tests (retrieved with --target=compose-e2e-tests)
# image is publised as docker/kube-compose-e2e-tests, and used for docker/orca e2e tests
FROM runbase AS compose-e2e-tests
ENTRYPOINT ["/e2e.test"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/e2e/e2e.test /e2e.test

# compose-installer (retrieved with --target=compose-installer)
FROM runbase AS compose-installer
ENTRYPOINT ["/installer"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/installer /installer

# compose-api-server (retrieved with --target=compose-api-server)
FROM runbase AS compose-api-server
ENTRYPOINT ["/api-server"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/api-server /api-server

# compose-api-server with instrumentation (retrieved with --target=compose-api-server-coverage)
FROM runbase AS compose-api-server-coverage
ENTRYPOINT ["/api-server-coverage"]
ADD e2e/api-server-coverage /
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/api-server.test /api-server.test

# instrumented runtime image (retrieved with --target=compose-controller-coverage)
FROM runbase AS compose-controller-coverage
ENTRYPOINT ["/compose-controller-coverage"]
ADD e2e/compose-controller-coverage /
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/compose-controller.test /compose-controller.test

# main runtime image (default target)
FROM runbase
ENTRYPOINT ["/compose-controller"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/compose-controller /compose-controller
