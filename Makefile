GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

IMAGE_NAME ?= mock-ai
IMAGE_TAG  ?= latest

.PHONY: build run test test-verbose test-race test-coverage lint lint-fix fmt vet clean help \
        docker-build docker-build-nonroot docker-run docker-run-nonroot

## help: Show this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

## build: Build the mock-ai binary
build:
	go build -o bin/mock-ai ./cmd/mock-ai

## run: Run the mock-ai server
run:
	go run ./cmd/mock-ai

## test: Run all tests
test:
	go test ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	go test -v ./...

## test-race: Run all tests with race detector
test-race:
	go test -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm -f coverage.out

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## lint-fix: Run golangci-lint with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format all Go files
fmt:
	gofmt -w .

## clean: Remove build artifacts
clean:
	rm -rf bin/

## docker-build: Build the simple Docker image (scratch-based)
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

## docker-build-nonroot: Build the non-root Docker image (alpine-based)
docker-build-nonroot:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG)-nonroot -f Dockerfile.nonroot .

## docker-run: Run the simple Docker image
docker-run:
	docker run --rm -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)

## docker-run-nonroot: Run the non-root Docker image
docker-run-nonroot:
	docker run --rm -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)-nonroot
