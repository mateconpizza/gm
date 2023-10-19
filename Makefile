# gomarks - simple bookmark manager
# See LICENSE file for copyright and license details.

NAME = gomarks
SRC = ./cmd/main.go
BIN = ./$(NAME)

.PHONY: all build run test vet clean

all: build

build: vet test
	@echo Building $(NAME)
	go build -o $(BIN) $(SRC)
	@echo

run:
	go run $(SRC)

test: vet
	@echo Testing $(NAME)
	go test -v ./...
	@echo

vet:
	@echo Checking code with go vet
	go vet ./...
	@echo

clean:
	@echo Cleaning up
	rm -f $(BIN)
	@echo
