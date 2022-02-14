VERSION := v0.0.1
HASH := $(shell git rev-parse --short HEAD)

# Protocol Buffer related vars
PROTOC_VERSION := 3.19.0
PROTOC_OS = linux
PROTOC_ARCH = x86_64
PROM_CLIENT_VERSION := 1.12.1
test:
	go test ./... -cover -race -count=1

cover:
	go test ./... -cover -race -count=1 -coverprofile cover.out
	go tool cover -html cover.out -o coverage.html

build:
	go build -o=agent -ldflags="\
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	" cmd/agent/main.go

build-dapper:
	go build -o=agent -ldflags="\
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Protocol=dapper' \
	" cmd/agent/main.go

build-algorand:
	go build -o=agent -ldflags="\
	-X 'agent/internal/pkg/global.Version=${VERSION}' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Protocol=algorand' \
	" cmd/agent/main.go

protogen:
	$(eval PROTOC_TMP := $(shell mktemp -d))
	rm -rf $(PWD)/tmp/include/google $(PWD)/tmp/go/io
	mkdir -p tmp/include && mkdir -p tmp/go
	
	cd $(PROTOC_TMP); curl -sSL https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip -o protoc.zip
	cd $(PROTOC_TMP); unzip protoc.zip && mv include/google $(PWD)/tmp/include/
	cd $(PROTOC_TMP); git clone https://github.com/prometheus/client_model.git && mv client_model/io/ $(PWD)/tmp/go/

	protoc \
	-I tmp/include -I tmp/go -I api/v1/proto \
	--go_out=./api/v1/model \
	--go-grpc_out=./api/v1/model \
	api/v1/proto/agent.proto