# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

NAME = gm
SRC = ./main.go
BIN = ./bin/$(NAME)

.PHONY: all build run test vet clean

all: full

full: vet lint test build

help:	## This help dialog.
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

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

debug: vet test
	@echo '>> Building $(NAME) with debugger'
	@go build -gcflags="all=-N -l" -o $(BIN)-debug $(SRC)

run: build
	@echo '>> Running $(NAME)'
	$(BIN)

test: vet
	@echo '>> Testing $(NAME)'
	@go test ./...
	@echo

test-verbose: vet
	@echo '>> Testing $(NAME) (verbose)'
	@go test -v ./...

vet:
	@echo '>> Checking code with go vet'
	@go vet ./...

clean:
	@echo '>> Cleaning up'
	rm -f $(BIN)
	go clean -cache

.PHONY: fmt
fmt:
	@echo '>> Formatting code'
	@gofumpt -l -w .

.PHONY: lint
lint: vet
	@echo '>> Linting code'
	@golangci-lint run ./...
	@codespell .

.PHONY: check
check:
	@echo '>> Linting everything'
	@golangci-lint run -p bugs -p error
