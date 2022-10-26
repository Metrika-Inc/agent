DBG_MAKEFILE ?=
ifeq ($(DBG_MAKEFILE),1)
    $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
    $(warning ***** $(shell date))
else
    # If we're not debugging the Makefile, don't echo recipes.
    MAKEFLAGS += -s
endif

# We don't need make's built-in rules.
MAKEFLAGS += --no-builtin-rules
# Be pedantic about undefined variables.
MAKEFLAGS += --warn-undefined-variables
.SUFFIXES:

# This version-strategy uses git tags to set the version string
VERSION ?= $(shell git describe --tags --always --dirty)
VERSION ?= v0.0.1unknown

HASH := $(shell git rev-parse --short HEAD)

# Protocol Buffer related vars
PROTOC_VERSION := 3.20.1
PROTOC_GEN_GO_GRPC_VERSION := v1.1
PROTOC_GEN_GO_VERSION := v1.5.2
PROTOC_OS := linux
PROTOC_ARCH := x86_64
PROM_CLIENT_VERSION := 1.12.1
OPENMETRICS_VERSION := 1.0.0

# List of supported blockchains by the agent
FLOW := flow
PROTOS = $(FLOW)

GOOS := linux
GOARCH := amd64

LINUX_GOARCHAR = amd64 arm64
DARWIN_GOARCHAR = amd64 arm64

OSAR = LINUX DARWIN

# Docker
DOCKER = $(shell which docker)
DOCKERFILE := ./docker/Dockerfile

PROTOBIND = protobind

.PHONY: all
all: build

.PHONY: linux-amd64-env
linux-amd64-env:
	$(eval GOOS:=linux)
	$(eval GOARCH:=amd64)

.PHONY: linux-arm64-env
linux-arm64-env:
	$(eval GOOS:=linux)
	$(eval GOARCH:=arm64)

.PHONY: osx-amd64-env
osx-amd64-env:
	$(eval GOOS:=darwin)
	$(eval GOARCH:=amd64)

.PHONY: osx-arm64-env
osx-arm64-env:
	$(eval GOOS:=darwin)
	$(eval GOARCH:=arm64)

.PHONY: test-%
test-%:
	go test -tags=$* ./... -cover -race -count=1 -coverprofile cover.out

.PHONY: test
test: $(foreach b,$(PROTOS),test-$(b))

.PHONY: cover-%
cover-%: test-%
	go tool cover -html cover.out -o coverage.html

.PHONY: $(PROTOBIND)
$(PROTOBIND):
	@cd $(PROTOBIND) && $(MAKE) install

.PHONY: generate-%
generate-%: $(PROTOBIND)
	MA_SRC_PATH=$(dir $(abspath $(lastword $(MAKEFILE_LIST)))) \
		CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go generate -tags=$* ./...

.PHONY: build-%-dbg
build-%-dbg: generate-%
	echo "Building Metrikad agent: GOOS: $(GOOS) GOARCH: $(GOARCH) PROTO: ${*}"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o=metrikad-${*}-$(GOOS)-$(GOARCH) -tags=${*} -ldflags=" \
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Blockchain=${*}' \
	" cmd/agent/main.go

.PHONY: build-%-strip
build-%-strip: generate-%
	echo "Building Metrikad agent: GOOS: $(GOOS) GOARCH: $(GOARCH) PROTO: ${*}"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -o=metrikad-${*}-$(GOOS)-$(GOARCH) -tags=${*} -ldflags=" \
	-s \
	-w \
	-linkmode=external \
	-extldflags=-Wl,-z,relro,-z,now \
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Blockchain=${*}' \
	" cmd/agent/main.go

.PHONY: checksum-%
checksum-%:
	sha256sum metrikad-${*}-$(GOOS)-$(GOARCH) > metrikad-${*}-$(GOOS)-$(GOARCH).sha256

.PHONY: docker-build-%
docker-build-%: generate-%
	echo "Building Metrikad Docker agent"
	$(DOCKER) build --platform $(GOOS)/$(GOARCH) -f $(DOCKERFILE) \
		-t metrikad-${*}:${VERSION} \
		--build-arg MA_PROTOCOL=${*} \
		--build-arg MA_VERSION=${VERSION} .

.PHONY: protogen
protogen:
	$(eval PROTOC_TMP := $(shell mktemp -d))
	rm -rf $(PWD)/tmp/include/google $(PWD)/tmp/go/io
	mkdir -p tmp/include tmp/go tmp/bin tmp/openmetrics api/v1/proto/openmetrics

	cd $(PROTOC_TMP); curl -sSL https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip -o protoc.zip
	cd $(PROTOC_TMP); unzip protoc.zip && mv include/google $(PWD)/tmp/include/
	cd $(PROTOC_TMP); git clone https://github.com/prometheus/client_model.git && mv client_model/io/ $(PWD)/tmp/go/
	cd $(PROTOC_TMP); git clone https://github.com/OpenObservability/OpenMetrics.git && mv OpenMetrics/proto/openmetrics_data_model.proto $(PWD)/tmp/openmetrics/openmetrics.proto

	echo '\noption go_package = "./;model";\n' >> $(PWD)/tmp/openmetrics/openmetrics.proto
	mv $(PWD)/tmp/openmetrics/openmetrics.proto api/v1/proto/openmetrics/
	mv $(PROTOC_TMP)/bin/protoc $(PWD)/tmp/bin/protoc
	GOBIN=$(PWD)/tmp/bin go install github.com/golang/protobuf/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(PWD)/tmp/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

	PATH=$(PWD)/tmp/bin:$$PATH protoc \
	-I tmp/include -I tmp/go -I api/v1/proto -I tmp/openmetrics \
	--go_opt=Mapi/v1/openmetrics/openmetrics.proto=agent/api/v1/model \
	--go_out=api/v1/model \
	--go-grpc_out=api/v1/model \
	api/v1/proto/agent.proto api/v1/proto/openmetrics/openmetrics.proto

.PHONY: fmt
fmt:
	go fmt ./...


.PHONY: build-all-echo
build-all-echo:
	echo "protogen $(PROTOBIND) $(foreach o,$(OSAR),$(foreach a,$($(o)_GOARCHAR),$(foreach p,$(PROTOS),$(shell echo $(o) | tr A-Z a-z)-$(a)-env build-$(shell echo $(o) | tr A-Z a-z)-$(a)-$(p) checksum-$(shell echo $(o) | tr A-Z a-z)-$(a)-$(p))))"

.PHONY: build-linux-amd64
build-linux-amd64: $(PROTOBIND) linux-amd64-env $(foreach b,$(PROTOS),build-$(b)-strip checksum-$(b))

.PHONY: build-linux-arm64
build-linux-arm64: $(PROTOBIND) linux-arm64-env $(foreach b,$(PROTOS),build-$(b)-strip checksum-$(b))

.PHONY: build
build: $(PROTOBIND) $(foreach b,$(PROTOS),build-$(b)-strip)

.PHONY: build-dbg
build-dbg: $(PROTOBIND) $(foreach b,$(PROTOS),build-$(b)-dbg)

.PHONY: docker-build-linux-amd64
docker-build-linux-amd64: $(PROTOBIND) linux-amd64-env $(foreach b,$(PROTOS),docker-build-$(b))

.PHONY: docker-build-linux-arm64
docker-build-linux-arm64: $(PROTOBIND) linux-arm64-env $(foreach b,$(PROTOS),docker-build-$(b))

.PHONY: clean-%
clean-%:
	@rm -rf metrikad-$*

.PHONY: clean
clean: $(foreach b,$(PROTOS),clean-$(b))
	@cd $(PROTOBIND) && $(MAKE) clean

