APP-BIN := ./bin/nomad-external-dns.bin

LAST_COMMIT := $(shell git rev-parse --short HEAD)
LAST_COMMIT_DATE := $(shell git show -s --format=%ci ${LAST_COMMIT})
VERSION := $(shell git describe --tags)
BUILDSTR := ${VERSION} (Commit: ${LAST_COMMIT_DATE} (${LAST_COMMIT}), Build: $(shell date +"%Y-%m-%d% %H:%M:%S %z"))

.PHONY: build
build: ## Build binary.
	go build -o ${APP-BIN} -ldflags="-X 'main.buildString=${BUILDSTR}'" ./cmd/

.PHONY: run
run: ## Run binary.
	mkdir -p ./data/events
	./${APP-BIN}

.PHONY: fresh
fresh: clean build run

.PHONY: clean
clean:
	rm -rf bin/${APP-BIN}
	go clean

.PHONY: lint
lint:
	docker run --rm -v $(pwd):/app -w /app golangci/golangci-lint:v1.43.0 golangci-lint run -v
