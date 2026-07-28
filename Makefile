.PHONY: build run test lint clean

BINARY=vocab

build:
	go build -o $(BINARY) ./cmd/vocab

run: build
	./$(BINARY)

test:
	go test ./... -v -race -count=1

LINT_BIN=$(shell go env GOPATH)/bin/golangci-lint

lint:
	@if [ -x $(LINT_BIN) ]; then $(LINT_BIN) run ./...; else echo "golangci-lint not installed, run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; go vet ./...; fi

clean:
	rm -f $(BINARY)
	rm -f coverage.out

coverage:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
