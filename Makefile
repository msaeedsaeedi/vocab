.PHONY: build run test lint clean

BINARY=vocab

build:
	go build -o $(BINARY) ./cmd/vocab

run: build
	./$(BINARY)

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
	rm -f coverage.out

coverage:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
