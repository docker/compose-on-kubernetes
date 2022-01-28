include common.mk

BINARIES = bin/compose-controller bin/api-server bin/installer bin/reconciliation-recorder bin/e2e_benchmark
TEST_PACKAGES := ${shell go list ./internal/... ./cmd/... ./api/... ./install/... | grep -v "_generated" | sed 's,github.com/docker/compose-on-kubernetes/,,' | uniq}
# Number of tests we expect to be skipped in e2e suite. Rule will fail if mismatch.
# We currently expect the two volume tests to fail on cluster without a default
# dynamic volume provisioner ('kubectl get storageclass' to check).
E2E_EXPECTED_SKIP ?= 2

VERBOSE_GO :=
ifeq ($(VERBOSE),true)
	VERBOSE_GO := -v
endif

EXEC_EXT :=
ifeq ($(OS),Windows_NT)
	EXEC_EXT := .exe
endif

LDFLAGS := "-s -w \
  -X ${PKG_NAME}/internal.GitCommit=${GITCOMMIT} \
  -X ${PKG_NAME}/internal.BuildTime=${BUILDTIME} \
  -X ${PKG_NAME}/internal.Version=${VERSION} \
  -X ${PKG_NAME}/install.imageRepoPrefix=${IMAGE_REPO_PREFIX}"

ifeq ($(DEBUG),true)
  GOBUILD_FLAGS := -gcflags "all=-N -l"
  LDFLAGS := "\
    -X ${PKG_NAME}/internal.GitCommit=${GITCOMMIT} \
    -X ${PKG_NAME}/internal.BuildTime=${BUILDTIME} \
    -X ${PKG_NAME}/internal.Version=${VERSION} \
    -X ${PKG_NAME}/install.imageRepoPrefix=${IMAGE_REPO_PREFIX}"
endif

.PHONY: all binaries check test e2e vendor run clean validate build-validate-image install uninstall e2e-binary
.DEFAULT: all

all: binaries test

FORCE:
	@test $$(go list) = "${PKG_NAME}" || \
			(echo "👹 Please correctly set up your Go build environment. This project must be located at <GOPATH>/src/${PKG_NAME}" && false)

clean: ## clean the binaries
	rm -Rf ./bin

bin/installer: cmd/installer FORCE
	@echo "🌟 $@"
	@CGO_ENABLED=0 go build $(VERBOSE_GO) $(GOBUILD_FLAGS) -ldflags=$(LDFLAGS) -o $@$(EXEC_EXT) ./$<

bin/%: cmd/% FORCE
	@echo "🌟 $@"
	@go build $(VERBOSE_GO) $(GOBUILD_FLAGS) -ldflags=$(LDFLAGS) -o $@$(EXEC_EXT) ./$<

e2e-binary: ## make the end to end test binary
	@echo " $@"
	@go test -c ./e2e $(VERBOSE_GO) -ldflags=$(LDFLAGS) -o e2e/e2e.test$(EXEC_EXT)

bin/%.test: cmd/% FORCE
	@echo "🌟 $@"
	@go test -c -covermode=atomic -coverpkg ${shell go list ./internal/... ./api/... ./install/... ./cmd/api-server/... ./cmd/compose-controller/... | tr -s '\n' ','} $(VERBOSE_GO) -ldflags=$(LDFLAGS) -o $@ ./$<

binaries: $(BINARIES) $(addsuffix .test,$(BINARIES)) ## build binaries

check test: $(addsuffix .check,$(TEST_PACKAGES)) ## run tests
	cat $(shell sh -c "find -name go-coverage.txt") > coverage.txt
	rm $(shell sh -c "find -name go-coverage.txt")

%.check:
	@echo "🌡 Check $*"
	@go test -ldflags=$(LDFLAGS) $(VERBOSE_GO) -cover -covermode=atomic -race  $(TESTFLAGS) -test.short -test.coverprofile="./$*/go-coverage.txt" "./$*"

check-licenses: ## Check the licenses for our dependencies
	go get -u github.com/frapposelli/wwhrd
	$(GOPATH)/bin/wwhrd check

install-ginkgo:
	GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.16.5
	go get -u github.com/onsi/gomega/...

e2e: ## Run the e2e tests
	IMAGE_REPO_PREFIX=$(IMAGE_REPO_PREFIX) ginkgo -v -p e2e/ -- -tag "$(TAG)" 2>&1 | tee e2e-test-output.txt
	grep SUCCESS e2e-test-output.txt | grep  -q "$(E2E_EXPECTED_SKIP) Skipped"

e2e-kind: ## Run the e2e tests
	KUBECONFIG="$(shell kind get kubeconfig-path --name="compose-on-kube")" IMAGE_REPO_PREFIX=$(IMAGE_REPO_PREFIX) ginkgo -v -p $(GINKGO_FLAGS) e2e/ -- -tag "$(TAG)" 2>&1 | tee e2e-test-output.txt
	grep SUCCESS e2e-test-output.txt | grep  -q "$(E2E_EXPECTED_SKIP) Skipped"

e2e-kind-circleci:
	docker rm compose-on-kube-e2e || echo "no existing compose-on-kube e2e container"
	docker create --name compose-on-kube-e2e -e IMAGE_REPO_PREFIX=$(IMAGE_REPO_PREFIX) -e KUBECONFIG=/kind-config --network=host ${IMAGE_REPO_PREFIX}e2e-tests:${TAG} -ginkgo.v -tag "$(TAG)"
	docker cp $(shell kind get kubeconfig-path --name="compose-on-kube") compose-on-kube-e2e:/kind-config
	docker start -a -i compose-on-kube-e2e
	docker cp compose-on-kube-e2e:/e2e ./e2e-coverage

e2e-benchmark-kind-circleci:
	docker rm compose-on-kube-e2e-benchmark || echo "no existing compose-on-kube e2e benchmark container"
	docker create --name compose-on-kube-e2e-benchmark -e IMAGE_REPO_PREFIX=$(IMAGE_REPO_PREFIX) -e TAG=$(TAG) -e KUBECONFIG=/kind-config --network=host ${IMAGE_REPO_PREFIX}e2e-benchmark:${TAG} --logs-namespace=benchmark -f report --total-stacks 50 --max-duration 7m
	docker cp $(shell kind get kubeconfig-path --name="compose-on-kube") compose-on-kube-e2e-benchmark:/kind-config
	docker start -a -i compose-on-kube-e2e-benchmark

e2e-kind-pods-info:
	kubectl --kubeconfig=$(shell kind get kubeconfig-path --name="compose-on-kube") get pods --all-namespaces

e2e-no-provisioning: e2e-binary ## run the e2e tests on an already provisionned cluster
	ginkgo -v -p e2e/e2e.test -- --skip-provisioning $(TEST_ARGS) 2>&1 | tee e2e-test-output.txt
	grep SUCCESS e2e-test-output.txt | grep -q "$(E2E_EXPECTED_SKIP) Skipped"

validate-vendor: ## validate vendoring
	./scripts/validate-vendor

vendor: ## do vendoring actions
	rm -rf ./vendor
	./scripts/dep-refresh

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

openapi:
	openapi-gen -i github.com/docker/compose-on-kubernetes/api/compose/v1beta1,github.com/docker/compose-on-kubernetes/api/compose/v1beta2,github.com/docker/compose-on-kubernetes/api/compose/v1alpha3,github.com/docker/compose-on-kubernetes/api/compose/impersonation,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/version,k8s.io/apimachinery/pkg/runtime -p github.com/docker/compose-on-kubernetes/api/openapi
