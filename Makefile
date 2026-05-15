.PHONY: test staged-test

test:
	go test ./...
	go vet ./...
	go build -o bin/codex-mcp-parity ./cmd/codex-mcp-parity

staged-test:
	go test ./cmd/codex-mcp-parity -run Staged -count=1 -v

