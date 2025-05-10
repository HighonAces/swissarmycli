.PHONY: build clean run test

# Binary name
BINARY_NAME=swissarmycli

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get

# Build directory
BUILD_DIR=./bin
VERSION=0.1.0

# Build the application
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/swissarmycli

# Run the application
run:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/swissarmycli
	$(BUILD_DIR)/$(BINARY_NAME)

# Install the application to the GOPATH
install:
	$(GOBUILD) -o $(GOPATH)/bin/$(BINARY_NAME) -v ./cmd/swissarmycli

# Clean build files
clean:
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Update dependencies
deps:
	$(GOGET) -u all
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) ./...

# Vet code
vet:
	$(GOVET) ./...

# Build for multiple platforms
build-all:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 -v ./cmd/swissarmycli
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe -v ./cmd/swissarmycli
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 -v ./cmd/swissarmycli

# Install the application to /usr/local/bin (builds first if necessary)
install-latest: build
	@echo "Installing latest build of $(BINARY_NAME) to /usr/local/bin..."
	./scripts/install_latest.sh # Assuming you created scripts/install_latest.sh
