.PHONY: build build-cli build-gui build-windows run run-cli run-gui test lint clean coverage

BINARY=vocab
GUI_BINARY=build/bin/vocab

build-cli:
	go build -o $(BINARY) ./cmd/vocab

build-gui:
	wails build -tags "webkit2_41 legacy_appindicator"

build-windows:
	wails build -platform windows/amd64 -tags "legacy_appindicator"

build: build-cli

run-cli: build-cli
	./$(BINARY)

run-gui:
	wails dev -tags "webkit2_41 legacy_appindicator"

run: run-cli

test:
	CGO_ENABLED=1 go test -tags "webkit2_41 legacy_appindicator" ./... -v -race -count=1

LINT_BIN=$(shell go env GOPATH)/bin/golangci-lint

lint:
	@if [ -x $(LINT_BIN) ]; then $(LINT_BIN) run ./...; else echo "golangci-lint not installed, run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; go vet ./...; fi

clean:
	rm -f $(BINARY)
	rm -f coverage.out
	rm -rf build/bin

coverage:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
