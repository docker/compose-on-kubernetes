ARG BUILD_BASE
ARG RUN_BASE
ARG KUBERNETES_VERSION

FROM ${RUN_BASE} AS runbase
RUN apk add ca-certificates --no-cache

FROM runbase AS unprivileged
RUN adduser -D compose
USER compose
WORKDIR /home/compose


FROM ${BUILD_BASE} AS build
ENV GO111MODULE=off

RUN apk add --no-cache \
  coreutils \
  make \
  git \
  curl

ARG GOPROXY

WORKDIR /go/src/github.com/docker/compose-on-kubernetes
COPY . .
ARG BUILDTIME
ARG GITCOMMIT
ARG VERSION
ARG IMAGE_REPO_PREFIX
ARG KUBERNETES_VERSION
ENV GITCOMMIT=$GITCOMMIT VERSION=$VERSION BUILDTIME=$BUILDTIME IMAGE_REPO_PREFIX=$IMAGE_REPO_PREFIX KUBERNETES_VERSION=$KUBERNETES_VERSION
ENV CGO_ENABLED=0
RUN make bin/compose-controller bin/compose-controller.test e2e-binary bin/installer bin/api-server bin/api-server.test bin/e2e_benchmark
RUN GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.16.5
RUN curl -fLO https://storage.googleapis.com/kubernetes-release/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubectl && \
  chmod +x ./kubectl && \
  mv ./kubectl /bin/kubectl

# e2e-tests (retrieved with --target=compose-e2e-tests)
# image is publised as docker/kube-compose-e2e-tests, and used for docker/orca e2e tests
FROM runbase AS compose-e2e-tests
ENTRYPOINT ["/ginkgo","-v", "-p", "--nodes=10", "/e2e.test", "--"]
COPY --from=build /go/bin/ginkgo /ginkgo
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/e2e/e2e.test /e2e.test
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/e2e/retrieve-coverage /retrieve-coverage
COPY --from=build /bin/kubectl /bin/kubectl

# e2e-benchmark
FROM runbase AS compose-e2e-benchmark
ENTRYPOINT ["/e2e_benchmark", "--kubeconfig=/kind-config", "--"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/e2e_benchmark /e2e_benchmark
COPY --from=build /bin/kubectl /bin/kubectl

# compose-installer (retrieved with --target=compose-installer)
FROM runbase AS compose-installer
ENTRYPOINT ["/installer"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/installer /installer

# compose-api-server (retrieved with --target=compose-api-server)
FROM unprivileged AS compose-api-server
ENTRYPOINT ["/api-server"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/api-server /api-server

# compose-api-server with instrumentation (retrieved with --target=compose-api-server-coverage)
FROM unprivileged AS compose-api-server-coverage
ENTRYPOINT ["/api-server-coverage"]
ADD e2e/api-server-coverage /
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/api-server.test /api-server.test

# instrumented runtime image (retrieved with --target=compose-controller-coverage)
FROM unprivileged AS compose-controller-coverage
ENTRYPOINT ["/compose-controller-coverage"]
ADD e2e/compose-controller-coverage /
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/compose-controller.test /compose-controller.test

# main runtime image (default target)
FROM unprivileged
ENTRYPOINT ["/compose-controller"]
COPY --from=build /go/src/github.com/docker/compose-on-kubernetes/bin/compose-controller /compose-controller
