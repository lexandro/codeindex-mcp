BINARY := codeindex-mcp
ifeq ($(OS),Windows_NT)
	BINARY := codeindex-mcp.exe
endif

.PHONY: build build-ast test test-ast run run-ast install install-ast clean check-cgo help

## build        Compile lightweight binary — no CGo, Go AST only (~17 MB)
build:
	go build -o $(BINARY) .

## build-ast    Compile full binary — CGo + tree-sitter, all 4 languages (~31 MB)
build-ast: check-cgo
	CGO_ENABLED=1 go build -tags ast -o $(BINARY) .

## test         Run tests (no CGo required)
test:
	go test ./...

## test-ast     Run tests including tree-sitter extractors (requires GCC)
test-ast: check-cgo
	CGO_ENABLED=1 go test -tags ast ./...

## run          Start server for the current directory (lightweight build)
run: build
	./$(BINARY) --root .

## run-ast      Start server with full AST symbol indexing (full build)
run-ast: build-ast
	./$(BINARY) --root . --ast

## install      Install lightweight binary to GOPATH/bin
install:
	go install .

## install-ast  Install full binary (with tree-sitter) to GOPATH/bin
install-ast: check-cgo
	CGO_ENABLED=1 go install -tags ast .

## clean        Remove the built binary
clean:
	rm -f $(BINARY)

## check-cgo    Verify that GCC is available (only needed for build-ast / test-ast)
check-cgo:
	@gcc --version > /dev/null 2>&1 || { \
		echo ""; \
		echo "ERROR: gcc not found. Required only for 'make build-ast'."; \
		echo ""; \
		echo "  Windows : powershell -ExecutionPolicy Bypass -File scripts/setup_build.ps1"; \
		echo "  Linux   : sudo apt install build-essential"; \
		echo "  macOS   : xcode-select --install"; \
		echo ""; \
		echo "For the lightweight build (Go AST only), 'make build' works without GCC."; \
		echo ""; \
		exit 1; \
	}

## help         List available targets
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'
