.PHONY: build test lint vet fmt run

build:
	go build ./cmd/delta

test:
	go test ./...

lint:
	go vet ./... && gofmt -l .

vet:
	go vet ./...

fmt:
	gofmt -w .

run:
	go run ./cmd/delta
