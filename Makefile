.PHONY: build run test clean help

BINARY_NAME=wrtp

build:
	go build -o bin/$(BINARY_NAME) cmd/$(BINARY_NAME)/main.go

run:
	go run cmd/$(BINARY_NAME)/main.go

test:
	go test ./...

clean:
	rm -rf bin/

help:
	@echo "Available targets:"
	@echo "  build       Build the application"
	@echo "  run         Run the application"
	@echo "  test        Run tests"
	@echo "  test-mode   Run in test mode (5s record + replay, requires sudo)"
	@echo "  clean       Clean build artifacts"
	@echo "  help        Show this help message"

test-mode:
	sudo go run cmd/$(BINARY_NAME)/main.go --test
