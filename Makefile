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
VERSION ?= v0.0.1

HASH := $(shell git rev-parse --short HEAD)

# Protocol Buffer related vars
PROTOC_VERSION := 3.19.0
PROTOC_GEN_GO_GRPC_VERSION := v1.1
PROTOC_GEN_GO_VERSION := v1.5.2
PROTOC_OS = linux
PROTOC_ARCH = x86_64
PROM_CLIENT_VERSION := 1.12.1
OPENMETRICS_VERSION := 1.0.0

# List of supported blockchains by the agent
DAPPER := dapper
ALGORAND := algorand
PROTOS = $(DAPPER) $(ALGORAND)

PROTOBIND = protobind

.PHONY: all
all: build

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
		go generate -tags=$* ./...

.PHONY: build-%
build-%: generate-%
	go build -o=metrikad-$* -tags=$* -ldflags="\
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Blockchain=$*' \
	" cmd/agent/main.go

protogen:
	$(eval PROTOC_TMP := $(shell mktemp -d))
	rm -rf $(PWD)/tmp/include/google $(PWD)/tmp/go/io
	mkdir -p tmp/include tmp/go tmp/bin tmp/openmetrics
	
	cd $(PROTOC_TMP); curl -sSL https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip -o protoc.zip
	cd $(PROTOC_TMP); unzip protoc.zip && mv include/google $(PWD)/tmp/include/
	cd $(PROTOC_TMP); git clone https://github.com/prometheus/client_model.git && mv client_model/io/ $(PWD)/tmp/go/
	cd $(PROTOC_TMP); git clone https://github.com/OpenObservability/OpenMetrics.git && mv OpenMetrics/proto/openmetrics_data_model.proto $(PWD)/tmp/openmetrics/openmetrics.proto

	echo '\noption go_package = "./;model";\n' >> $(PWD)/tmp/openmetrics/openmetrics.proto
	mv $(PWD)/tmp/openmetrics/openmetrics.proto api/v1/proto/
	mv $(PROTOC_TMP)/bin/protoc $(PWD)/tmp/bin/protoc
	GOBIN=$(PWD)/tmp/bin go install github.com/golang/protobuf/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(PWD)/tmp/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

	PATH=$(PWD)/tmp/bin:$$PATH protoc \
	-I tmp/include -I tmp/go -I api/v1/proto -I tmp/openmetrics \
	--go_opt=Mapi/v1/openmetrics.proto=agent/api/v1/model \
	--go_out=api/v1/model \
	--go-grpc_out=api/v1/model \
	api/v1/proto/agent.proto api/v1/proto/openmetrics.proto

.PHONY: build
build: $(PROTOBIND) $(foreach b,$(PROTOS),build-$(b))

.PHONY: clean-%
clean-%:
	@rm -rf metrikad-$*

.PHONY: clean
clean: $(foreach b,$(PROTOS),clean-$(b))
	@cd $(PROTOBIND) && $(MAKE) clean
