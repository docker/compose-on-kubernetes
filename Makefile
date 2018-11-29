include common.mk

BINARIES = bin/compose-controller bin/api-server bin/installer bin/updater bin/reconciliation-recorder bin/e2e_benchmark
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
endif

.PHONY: all binaries check test e2e vendor run clean validate build-validate-image install uninstall e2e-binary
.DEFAULT: all

all: binaries test

FORCE:
	@test $$(go list) = "${PKG_NAME}" || \
			(echo "👹 Please correctly set up your Go build environment. This project must be located at <GOPATH>/src/${PKG_NAME}" && false)

clean: ## clean the binaries
	rm -Rf ./bin

bin/%: cmd/% FORCE
	@echo "🌟 $@"
	@go build $(VERBOSE_GO) $(GOBUILD_FLAGS) -ldflags=$(LDFLAGS) -o $@$(EXEC_EXT) ./$<

e2e-binary: ## make the end to end test binary
	@echo "🌟 $@"
	@go test -c ./e2e $(VERBOSE_GO) -ldflags=$(LDFLAGS) -o e2e/e2e.test$(EXEC_EXT)

e2e-migration-binary: ## make the migration test binary
	@echo "🌟 $@"
	@go test -c ./e2e_migration $(VERBOSE_GO) -ldflags=$(LDFLAGS) -o e2e_migration/e2e_migration.test$(EXEC_EXT)

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
	go get -u github.com/onsi/ginkgo/ginkgo
	go get -u github.com/onsi/gomega/...

e2e: e2e-binary ## Run the e2e tests
	ginkgo -v -p e2e/e2e.test -- -tag "$(TAG)" 2>&1 | tee e2e-test-output.txt
	grep SUCCESS e2e-test-output.txt | grep  -q "$(E2E_EXPECTED_SKIP) Skipped"

e2e-migration: e2e-migration-binary ## Run the e2e tests
	ginkgo -v -p e2e_migration/e2e_migration.test -- -tag "$(TAG)" $(TEST_ARGS)

e2e-no-provisioning: e2e-binary ## run the e2e tests on an already provisionned cluster
	ginkgo -v -p e2e/e2e.test -- --skip-provisioning $(TEST_ARGS) 2>&1 | tee e2e-test-output.txt
	grep SUCCESS e2e-test-output.txt | grep -q "$(E2E_EXPECTED_SKIP) Skipped"

validate-vendor: ## validate vendoring
	./scripts/validate-vendor

dep: ## refresh dependencies
	./scripts/dep-refresh

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

openapi:
	openapi-gen -i github.com/docker/compose-on-kubernetes/api/compose/v1beta1,github.com/docker/compose-on-kubernetes/api/compose/v1beta2,github.com/docker/compose-on-kubernetes/api/compose/impersonation,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/version,k8s.io/apimachinery/pkg/runtime -p github.com/docker/compose-on-kubernetes/api/openapi
