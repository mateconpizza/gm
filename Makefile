# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

NAME = gomarks
SRC = ./main.go
BIN = ./bin/$(NAME)

.PHONY: all build run test vet clean

all: full

full: fmt vet lint test build

build: vet test
	@echo '>> Building $(NAME)'
	go build -ldflags "-s -w" -o $(BIN) $(SRC)
	@echo

beta: vet test
	@echo '>> Building $(NAME)'
	go build -o $(BIN)-beta $(SRC)
	@echo

debug: vet test
	@echo '>> Building $(NAME) with debugger'
	go build -gcflags="all=-N -l" -o $(BIN)-debug $(SRC)
	@echo

run: build
	@echo '>> Running $(NAME)'
	$(BIN)

test: vet
	@echo '>> Testing $(NAME)'
	go test ./...
	@echo

test-verbose: vet
	@echo '>> Testing $(NAME) (verbose)'
	go test -v ./...
	@echo

vet:
	@echo '>> Checking code with go vet'
	go vet ./...
	@echo

clean:
	@echo '>> Cleaning up'
	rm -f $(BIN)
	go clean -cache
	@echo

.PHONY: fmt
fmt:
	@echo '>> Formatting code'
	gofumpt -l -w .
	@echo

.PHONY: lint
lint: vet
	@echo '>> Linting code'
	golangci-lint run ./...
	@echo

.PHONY: check
check:
	@echo '>> Linting everything'
	golangci-lint run -p bugs -p error
	@echo
