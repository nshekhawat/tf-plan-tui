BINARY_NAME=tf-plan-tui
VERSION?=0.1.0
BUILD_DIR=bin
INSTALL_PATH=$(GOPATH)/bin

.PHONY: build test install clean run lint help

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run all tests
test:
	@echo "Running tests..."
	@go test ./... -v -race -cover

## test-short: Run tests without race detector
test-short:
	@go test ./... -v

## install: Build and install to GOPATH/bin
install: build
	@echo "Installing to $(INSTALL_PATH)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installed $(INSTALL_PATH)/$(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f .tf-plan-tui.tfplan
	@echo "Cleaned"

## run: Run with test data
run:
	@cat testdata/simple_plan.json | go run . -

## run-destructive: Run with destructive test plan
run-destructive:
	@cat testdata/destructive_plan.json | go run . -

## run-modules: Run with module test plan
run-modules:
	@cat testdata/module_plan.json | go run . -

## lint: Run linter
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	@golangci-lint run ./...

## fmt: Format code
fmt:
	@go fmt ./...

## vet: Run go vet
vet:
	@go vet ./...

## coverage: Generate test coverage report
coverage:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## help: Show this help
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'

.DEFAULT_GOAL := help
