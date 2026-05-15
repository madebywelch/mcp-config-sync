.PHONY: test staged-test

test:
	go test ./...
	go vet ./...
	go build -o bin/mcp-config-sync ./cmd/mcp-config-sync

staged-test:
	go test ./cmd/mcp-config-sync -run Staged -count=1 -v
