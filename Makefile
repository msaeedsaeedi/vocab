.PHONY: build build-windows run test lint clean coverage

BINARY=vocab
GOCACHE ?= $(CURDIR)/.cache/go-build

build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/vocab

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui -X main.version=dev" -o $(BINARY).exe ./cmd/vocab

run: build
	./$(BINARY)

test:
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		GOCACHE=$(GOCACHE) go test ./... -v -race -count=1; \
	else \
		echo "cgo unavailable: -race requires cgo; running tests without -race"; \
		GOCACHE=$(GOCACHE) go test ./... -v -count=1; \
	fi

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
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		GOCACHE=$(GOCACHE) go test ./... -race -count=1 -coverprofile=coverage.out; \
	else \
		echo "cgo unavailable: -race requires cgo; running coverage without -race"; \
		GOCACHE=$(GOCACHE) go test ./... -count=1 -coverprofile=coverage.out; \
	fi
	go tool cover -html=coverage.out -o coverage.html
