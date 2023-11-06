# Copyright (c) 2017-2021, Habana Labs. All rights reserved.
TAG := $(shell git describe --abbrev=0 --tags --always)
HASH := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d.%H:%M:%S)

DOCKER ?= docker
MKDIR  ?= mkdir
DIST_DIR ?= $(CURDIR)/dist
LOCAL_REGISTRY ?= ""

LIB_NAME := habana-container-runtime
LIB_VERSION := 0.0.1
PKG_REV := 1

TOOLKIT_VERSION := 1.3.0
GOLANG_VERSION  := 1.21

# Go CI related commands
build:
	@go build  -o ${LIB_NAME}

test:
	cd src/ && go test ./... -coverprofile=coverage.out

coverage:
	go tool cover -func coverate.out | grep "total": | awk '{print ((int$$3) > 80) != 1}'

check-format:
	test -z $$(go fmt ./src/...)

static-check:
	@cd src/ && \
	golangci-lint run ./...

#######################################

# Supported OSs by architecture
AMD64_TARGETS := ubuntu20.04 ubuntu22.04 ubuntu18.04 debian10.10
X86_64_TARGETS := centos7 centos8 rhel7 rhel8 amazonlinux1 amazonlinux2

# amd64 targets
AMD64_TARGETS := $(patsubst %, %-amd64, $(AMD64_TARGETS))
$(AMD64_TARGETS): ARCH := amd64
$(AMD64_TARGETS): %: --%
docker-amd64: $(AMD64_TARGETS)

# x86_64 targets
X86_64_TARGETS := $(patsubst %, %-x86_64, $(X86_64_TARGETS))
$(X86_64_TARGETS): ARCH := x86_64
$(X86_64_TARGETS): %: --%
docker-x86_64: $(X86_64_TARGETS)

# Default variables for all private '--' targets below.
# One private target is defined for each OS we support.
--%: TARGET_PLATFORM = $(*)
--%: VERSION = $(patsubst $(OS)%-$(ARCH),%,$(TARGET_PLATFORM))
--%: BASEIMAGE = $(OS):$(VERSION)

--%: BUILDIMAGE = habana/$(LIB_NAME)/$(OS)$(VERSION)-$(ARCH)
--%: DOCKERFILE = $(CURDIR)/docker/Dockerfile.$(OS)
--%: ARTIFACTS_DIR = $(DIST_DIR)/$(OS)$(VERSION)/$(ARCH)
--%: docker-build-%
	@

# private OS targets with defaults
--ubuntu%: OS := ubuntu
--debian%: OS := debian
--centos%: OS := centos
--amazonlinux%: OS := amazonlinux

--rhel%: OS := centos
--rhel%: VERSION = $(patsubst rhel%-$(ARCH),%,$(TARGET_PLATFORM))
--rhel%: ARTIFACTS_DIR = $(DIST_DIR)/rhel$(VERSION)/$(ARCH)

docker-build-%:
	@echo "Building for $(TARGET_PLATFORM)"
	docker pull --platform=linux/$(ARCH) $(LOCAL_REGISTRY)$(BASEIMAGE)
	DOCKER_BUILDKIT=1 \
	$(DOCKER) build \
	    --progress=plain \
	    --build-arg BASEIMAGE=$(LOCAL_REGISTRY)$(BASEIMAGE) \
	    --build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
	    --build-arg TOOLKIT_VERSION="$(TOOLKIT_VERSION)" \
	    --build-arg PKG_VERS="$(LIB_VERSION)" \
	    --build-arg PKG_REV="$(PKG_REV)" \
	    --tag $(BUILDIMAGE) \
	    --file $(DOCKERFILE) .
	$(DOCKER) run \
	    -e DISTRIB \
	    -e SECTION \
	    -v $(ARTIFACTS_DIR):/dist \
	    $(BUILDIMAGE)
