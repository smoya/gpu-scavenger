BIN_DIR = ./bin
GOLANGCILINT_VERSION = 1.39.0

.PHONY: all
all: lint build

$(BIN_DIR)/golangci-lint: $(BIN_DIR)
	@wget -O - -q https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | BINDIR=$(@D) sh -s v$(GOLANGCILINT_VERSION) > /dev/null 2>&1

$(BIN_DIR):
	mkdir bin

.PHONY: lint
lint: $(BIN_DIR)/golangci-lint
	$(BIN_DIR)/golangci-lint run

.PHONY: build
build:
	go build -o bin/gpu-scavenger main.go

docker-build:
	docker build -t smoya/gpu-scavenger:latest .

docker-push: docker-build
	docker push smoya/gpu-scavenger:latest

