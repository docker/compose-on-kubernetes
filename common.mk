PKG_NAME = github.com/docker/compose-on-kubernetes
REGISTRY_ORG ?= docker
IMAGE_REPO_PREFIX ?= $(REGISTRY_ORG)/kube-compose-dev-
TAG ?= latest

ifeq ($(GITCOMMIT),)
  override GITCOMMIT := ${shell git rev-parse --short HEAD 2> /dev/null || echo "unknown-commit"}
  ifneq ($(strip $(shell git status --porcelain 2> /dev/null)),)
    override GITCOMMIT := $(addsuffix "-dirty", ${GITCOMMIT})
  endif
endif
ifeq ($(VERSION),)
  override VERSION := ${shell git describe --dirty --tags 2> /dev/null || echo "unknown-version"}
endif

ifeq ($(BUILDTIME),)
  override BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2> /dev/null)
endif
ifeq ($(BUILDTIME),)
  override BUILDTIME := unknown-buildtime
  $(warning unable to set BUILDTIME. Set the value manually)
endif
