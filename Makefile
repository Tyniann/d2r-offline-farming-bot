APP_NAME := d2rbot
CMD_PATH := ./cmd/d2rbot
BIN_DIR  := ./bin

.PHONY: all build run test lint fmt tidy clean tools

all: build

build:
	go build -o $(BIN_DIR)/$(APP_NAME).exe $(CMD_PATH)

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
