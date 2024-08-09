# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

PROJECT_NAME	:= gomarks
BINARY_NAME 	:= gm
GOBIN_PATH		:= ./bin
BINARY				:= $(GOBIN_PATH)/$(BINARY_NAME)
SRC 					:= ./main.go
INSTALL_DIR		:= /usr/local/bin
LDFLAGS				:= "-s -w"

all: full

full: deps build

deps:
	@go mod tidy

build: ## Generate bin
	@echo '>> Building $(PROJECT_NAME)'
	@CGO_ENABLED=1 go build -ldflags='-s -w' -o $(BINARY) $(SRC)

debug: test ## Generate bin with debugger
	@echo '>> Building $(BINARY_NAME) with debugger'
	@go build -gcflags='all=-N -l' -o $(BINARY)-debug $(SRC)

test: check ## Test
	@echo '>> Testing $(BINARY_NAME)'
	@go test ./...
	@echo

vtest: ## Test with verbose
	@echo '>> Testing $(BINARY_NAME) (verbose)'
	@go test -v ./...

lint: ## Lint code with 'golangci-lint'
	@echo '>> Linting code'
	@go vet ./...
	golangci-lint run ./...

check: ## Lint code with 'golangci-lint' and 'codespell'
	@echo '>> Checking code with linters'
	golangci-lint run -p bugs -p error
	codespell .

clean: ## Clean cache
	@echo '>> Cleaning bin'
	rm -rf $(GOBIN_PATH)

cleanall: clean ## clean cache
	@echo '>> Cleaning cache'
	go clean -cache

install: ## Install on system
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY_NAME)
	chmod 755 $(INSTALL_DIR)/$(BINARY_NAME)
	@echo '>> $(BINARY_NAME) has been installed on your device'

uninstall: ## Uninstall from system
	rm -rf $(GOBIN_PATH)
	rm -rf $(INSTALL_DIR)/$(BINARY_NAME)
	@echo '>> $(BINARY_NAME) has been removed from your device'

.PHONY: all build debug test clean full check lint
