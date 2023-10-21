# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

NAME = gomarks
SRC = ./cmd/main.go
BIN = ./$(NAME)

.PHONY: all build run test vet clean

all: full

full: vet lint fmt test build

build: vet test
	@echo '>> Building $(NAME)'
	go build -o $(BIN) $(SRC)
	@echo

run: build
	@echo '>> Running $(NAME)'
	$(BIN)

test: vet
	@echo '>> Testing $(NAME)'
	go test -cover ./...
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
	@echo

.PHONY: fmt
fmt:
	@echo '>> Formatting code'
	go fmt ./...
	@echo

.PHONY: lint
lint: vet
	@echo '>> Linting code'
	golangci-lint run ./...
	@echo
