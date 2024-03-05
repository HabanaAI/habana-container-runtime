# Copyright (c) 2017-2021, Habana Labs. All rights reserved.
TAG := $(shell git describe --abbrev=0 --tags --always)
HASH := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d.%H:%M:%S)

DOCKER ?= docker
MKDIR  ?= mkdir
DIST_DIR ?= $(CURDIR)/dist
LOCAL_REGISTRY ?= ""

RUNTIME_BINARY := habana-container-runtime
HOOK_BINARY := habana-container-hook
CLI_BINARY := habana-container-cli

LIB_VERSION := 0.0.1
PKG_REV := 1

TOOLKIT_VERSION := 1.3.0
GOLANG_VERSION  := 1.21.0

# # Go CI related commands
build-binary: clean build-runtime build-hook build-cli

build-runtime:
	@echo "Building $(RUNTIME_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${RUNTIME_BINARY} ./cmd/habana-container-runtime/
	@CGO_ENABLED=0 GOARCH=386 GOOS=linux go build  -o dist/linux_386/${RUNTIME_BINARY} ./cmd/habana-container-runtime/
	@CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build  -o dist/linux_arm64/${RUNTIME_BINARY} ./cmd/habana-container-runtime/

build-hook:
	@echo "Building $(HOOK_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${HOOK_BINARY} ./cmd/habana-container-runtime-hook/
	@CGO_ENABLED=0 GOARCH=386 GOOS=linux go build  -o dist/linux_386/${HOOK_BINARY} ./cmd/habana-container-runtime-hook/
	@CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build  -o dist/linux_arm64/${HOOK_BINARY} ./cmd/habana-container-runtime-hook/

build-cli:
	@echo "Building $(CLI_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${CLI_BINARY} ./cmd/habana-container-cli/
	@CGO_ENABLED=0 GOARCH=386 GOOS=linux go build  -o dist/linux_386/${CLI_BINARY} ./cmd/habana-container-cli/
	@CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build  -o dist/linux_arm64/${CLI_BINARY} ./cmd/habana-container-cli/

clean:
	go clean > /dev/null
	rm -rf dist/*

test:
	@go test ./... -coverprofile=coverage.out

coverage:
	@go tool cover -func coverage.out | grep "total:" | awk '{print  ((int($$3) > 80) != 1)}'

check-format:
	@test -z $$(go fmt ./...)

lint:
	@golangci-lint run ./...

tidy:
	@go mod tidy && go mod vendor

# Build the binaries in all available architectures.
build:
	docker run --rm --privileged \
		-v $$PWD:/go/src/github.com/HabanaAI/habana-container-runtime \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-w /go/src/github.com/HabanaAI/habana-container-runtime \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-e DOCKER_REGISTRY \
		goreleaser/goreleaser build --snapshot --clean

# Build binaries, create archives and OS packages and uploads all artifacts to github repo
release:
	docker run --rm \
		-v $$PWD:/go/src/github.com/HabanaAI/habana-container-runtime \
		-w /go/src/github.com/HabanaAI/habana-container-runtime \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-e DOCKER_REGISTRY \
		goreleaser/goreleaser release --clean --snapshot
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

--%: BUILDIMAGE = habana/habana-container-runtime/$(OS)$(VERSION)-$(ARCH)
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
		--build-arg ARCH=$(ARCH) \
	    --tag $(BUILDIMAGE) \
	    --file $(DOCKERFILE) .
	$(DOCKER) run \
	    -e DISTRIB \
	    -e SECTION \
	    -v $(ARTIFACTS_DIR):/dist \
	    $(BUILDIMAGE)
