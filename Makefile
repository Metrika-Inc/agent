VERSION := v0.0.1
HASH := $(shell git rev-parse --short HEAD)

test:
	go test ./... -cover -race -count=1

cover:
	go test ./... -cover -race -count=1 -coverprofile cover.out
	go tool cover -html cover.out -o coverage.html

build:
	go build -o=agent -ldflags="\
	-X 'agent/internal/pkg/global.Version=v0.0.1' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	" cmd/agent/main.go

build-dapper:
	go build -o=agent -ldflags="\
	-X 'agent/internal/pkg/global.Version=v0.0.1' \
	-X 'agent/internal/pkg/global.CommitHash=${HASH}' \
	-X 'agent/internal/pkg/global.Protocol=dapper' \
	" cmd/agent/main.go