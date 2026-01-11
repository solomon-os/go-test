.PHONY: build test cover lint clean

build:
	go build -o main ./cmd/drift-detector

test:
	go test ./... -v

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

clean:
	rm -f main coverage.out coverage.html
