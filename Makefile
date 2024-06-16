# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

NAME = gm## name
SRC = ./main.go## source
BIN = ./bin/$(NAME)## binary

.PHONY: all build run test vet clean full

all: full

full: vet lint test build

build: vet test	## Generate bin
	@echo '>> Building $(NAME)'
	@go build -ldflags "-s -w" -o $(BIN) $(SRC)

build-all: vet test	## Generate bin and bin with debugger
	@echo '>> Building $(NAME)'
	@echo '>> Building $(NAME) with debugger'
	@go build -ldflags "-s -w" -o $(BIN) $(SRC)
	@go build -gcflags="all=-N -l" -o $(BIN)-debug $(SRC)

beta: vet test
	@echo '>> Building $(NAME)'
	@go build -o $(BIN)-beta $(SRC)

debug: vet test ## Generate bin with debugger
	@echo '>> Building $(NAME) with debugger'
	@go build -gcflags="all=-N -l" -o $(BIN)-debug $(SRC)

run: build ## Run
	@echo '>> Running $(NAME)'
	$(BIN)

test: vet ## Test
	@echo '>> Testing $(NAME)'
	@go test ./...
	@echo

test-verbose: vet ## Test with verbose
	@echo '>> Testing $(NAME) (verbose)'
	@go test -v ./...

vet: ## Check code
	@echo '>> Checking code with go vet'
	@go vet ./...

clean: ## Clean cache
	@echo '>> Cleaning up'
	rm -f $(BIN)
	go clean -cache

.PHONY: fmt
fmt: ## Format code with 'gofumpt'
	@echo '>> Formatting code'
	@gofumpt -l -w .

.PHONY: lint
lint: vet ## Lint code with 'golangci-lint'
	@echo '>> Linting code'
	@golangci-lint run ./...
	@codespell .

.PHONY: check
check: ## Lint code with 'golangci-lint' and 'codespell'
	@echo '>> Linting everything'
	@golangci-lint run -p bugs -p error
