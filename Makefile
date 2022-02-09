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

# List of supported blockchains by the agent
DAPPER := dapper
ALGORAND := algorand
PROTOS = $(DAPPER) $(ALGORAND)

PROTOBIND = protobind

.PHONY: all
all: build

.PHONY: cover-%
cover-%:
	go test -tags=$* ./... -cover -race -count=1 -coverprofile cover.out
	go tool cover -html cover.out -o coverage.html

.PHONY: test-%
test-%:
	go test -tags=$* ./... -cover -race -count=1

.PHONY: test
test: $(foreach b,$(PROTOS),test-$(b))

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
	-X 'agent/internal/pkg/global.Protocol=$*' \
	" cmd/agent/main.go

.PHONY: build
build: $(PROTOBIND) $(foreach b,$(PROTOS),build-$(b))

.PHONY: clean-%
clean-%:
	@rm -rf metrikad-$*

.PHONY: clean
clean: $(foreach b,$(PROTOS),clean-$(b))
	@cd $(PROTOBIND) && $(MAKE) clean
