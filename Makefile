VERSION := v0.0.1
HASH := $(shell git rev-parse --short HEAD)

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
	protoc -I api/v1/proto --go_out=./api/v1/model \
	--go-grpc_out=./api/v1/model \
	api/v1/proto/agent.proto

wip_protogen:
	PATH=/home/tomas/client_model/tmp/bin:$$PATH protoc \
	-I tmp/include -I tmp/go -I api/v1/proto \
	--go_out=./api/v1/model \
	--go-grpc_out=./api/v1/model \
	api/v1/proto/agent.proto