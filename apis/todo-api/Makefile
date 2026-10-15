# Developer shortcuts for the todo API. Run `make` or `make help` to list them.
# Everything works from this directory; the Go module root is two levels up.

MODULE_ROOT := $(abspath ../..)
BIN         := bin/todo-api
IMAGE       ?= todo-api
PORT        ?= 8080

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: run
run: ## Run the server (override with PORT=9000 TODO_API_KEYS=key)
	PORT=$(PORT) go run ./cmd/server

.PHONY: build
build: ## Build a static binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

.PHONY: test
test: ## Run the tests
	go test ./...

.PHONY: race
race: ## Run the tests with the race detector, uncached
	go test -race -count=1 ./...

.PHONY: cover
cover: ## Run the tests with coverage and print per-function numbers
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1
	@echo "Open the HTML report with: go tool cover -html=coverage.out"

.PHONY: bench
bench: ## Run the benchmarks
	go test -run '^$$' -bench . -benchmem ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the code
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-formatted
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "needs gofmt:"; echo "$$unformatted"; exit 1; fi

.PHONY: check
check: fmt-check vet race ## Everything CI would run: format, vet and race tests

.PHONY: docker-build
docker-build: ## Build the container image
	docker build -f Dockerfile -t $(IMAGE) $(MODULE_ROOT)

.PHONY: docker-run
docker-run: ## Run the container image on the chosen PORT
	docker run --rm -p $(PORT):8080 -e TODO_API_KEYS=$${TODO_API_KEYS:-} $(IMAGE)

.PHONY: clean
clean: ## Remove build and coverage artifacts
	rm -rf bin coverage.out
