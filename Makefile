APP_NAME := d2rbot
CMD_PATH := ./cmd/d2rbot
BIN_DIR  := ./bin
VERSION  := $(shell grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Version=$(VERSION) -X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Commit=$(COMMIT)

.PHONY: all build release run test lint fmt tidy clean tools

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME).exe $(CMD_PATH)

release:
	powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version $(VERSION)

run:
	go run $(CMD_PATH)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	goimports -w .
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
