BINARY := pr-tree
PKG := ./cmd/pr-tree
GO_SOURCES := $(shell find cmd internal -name '*.go')

# Default goal: build the binary.
.DEFAULT_GOAL := $(BINARY)

# Build the pr-tree binary. Rebuilds only when a Go source changes.
$(BINARY): $(GO_SOURCES)
	go build -o $(BINARY) $(PKG)

# Run the full test suite.
test:
	go test ./...

# Install pr-tree into your Go bin directory (go env GOBIN, else GOPATH/bin).
install:
	go install $(PKG)

# Remove the locally built binary.
clean:
	rm -f $(BINARY)

.PHONY: test install clean
