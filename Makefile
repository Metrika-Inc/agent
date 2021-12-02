test:
	go test ./... -cover -race -count=1

cover:
	go test ./... -cover -race -count=1 -coverprofile cover.out
	go tool cover -html cover.out -o coverage.html