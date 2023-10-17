build:
	go build -o gomarks

run:
	go run main.go

test:
	go build -o gomarks
	gomarks -test
