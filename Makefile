.PHONY: build test run fmt vet

build:
	go build -o bin/jiuwen-auth-adapter ./cmd/server

test:
	go test ./...

run:
	go run ./cmd/server

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...
