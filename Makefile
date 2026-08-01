.PHONY: build build-windows run test integration-test lint clean coverage

BINARY=vocab
GOCACHE ?= $(CURDIR)/.cache/go-build

build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/vocab

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui -X main.version=dev" -o $(BINARY).exe ./cmd/vocab

run: build
	./$(BINARY)

test:
	GOCACHE=$(GOCACHE) go test ./... -v -race -count=1

# -race amplifies integrity checks on the large Lexicon bundle ~45x; the
# integration test is I/O-bound against big files, so run it without -race.
integration-test:
	VOCAB_LEXICON_INTEGRATION=1 GOCACHE=$(GOCACHE) go test ./internal/dataset -v -count=1

LINT_BIN=$(shell go env GOPATH)/bin/golangci-lint

lint:
	@if [ -x $(LINT_BIN) ]; then $(LINT_BIN) run ./...; else echo "golangci-lint not installed, run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; go vet ./...; fi

preview:
	go build -o $(BINARY) ./cmd/vocab
	./$(BINARY) -preview

clean:
	rm -f $(BINARY)
	rm -f $(BINARY).exe
	rm -f coverage.out

coverage:
	GOCACHE=$(GOCACHE) go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
