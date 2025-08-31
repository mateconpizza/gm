# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

PROJECT_NAME	:= gomarks
BINARY_NAME 	:= gm
BIN_DIR		:= $(CURDIR)/bin
BIN_PATH	:= $(BIN_DIR)/$(BINARY_NAME)
MAIN_SRC	:= $(CURDIR)/main.go
INSTALL_DIR	:= /usr/local/bin
LDFLAGS		:= -s -w
FN		?= .

full: build

# Target to build everything
all: lint check test build

# Build the binary
build:
	@echo '>> Building $(PROJECT_NAME)'
	@go build -ldflags='$(LDFLAGS)' -o $(BIN_PATH) $(MAIN_SRC)
	@echo '>> Binary built at $(BIN_PATH)'

# Build the binary with debugger
debug: test
	@echo '>> Building $(BINARY_NAME) with debugger'
	@go build -gcflags='all=-N -l' -o $(BIN_PATH)-debug $(MAIN_SRC)

# Run tests
test:
	@echo '>> Testing $(BINARY_NAME)'
	@go test ./...
	@echo

# Run tests with gotestsum
testsum:
	@echo '>> Testing $(BINARY_NAME)'
	@gotestsum --format pkgname --hide-summary=skipped --format-icons codicons

# Run tests with verbose mode on
vtest:
	@echo '>> Testing $(BINARY_NAME) (verbose)'
	@go test -v ./...

# Run tests for a specific function
testfn:
	@echo '>> Testing function $(FN)'
	@go test -run $(FN) ./...

# Run tests for a specific function with verbose
vtestfn:
	@echo '>> Testing function $(FN)'
	@go test -v -run $(FN) ./...

# Benchmark code
bench:
	@echo '>> Benchmark'
	@go test -run='^$$' -bench=. ./... | grep -v "PASS" | grep -v "ok" | grep -v "?"

# Lint code with 'golangci-lint'
lint:
	@echo '>> Linting code'
	@go vet ./...
	golangci-lint run ./...

# Lint code with 'golangci-lint' and 'codespell'
check:
	@echo '>> Checking code with linters'
	golangci-lint run -p bugs -p error
	codespell .

# Clean binary directories
clean:
	@echo '>> Cleaning bin'
	rm -rf $(BIN_DIR)

# Clean caches
cleanall: clean
	@echo '>> Cleaning cache'
	go clean -cache

# Install the binary to the system
install:
	mkdir -p $(INSTALL_DIR)
	cp $(BIN_PATH) $(INSTALL_DIR)/$(BINARY_NAME)
	chmod 755 $(INSTALL_DIR)/$(BINARY_NAME)
	@echo '>> $(BINARY_NAME) has been installed on your device'

# Uninstall the binary from the system
uninstall:
	rm -rf $(BIN_DIR)
	rm -rf $(INSTALL_DIR)/$(BINARY_NAME)
	@echo '>> $(BINARY_NAME) has been removed from your device'

.PHONY: all build debug test clean full check lint testfn
