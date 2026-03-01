.PHONY: tools generate build build-windows run test test-calc dev clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Install required dev tools and resolve deps (run once after clone)
tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/air-verse/air@latest
	go mod tidy

# Compile all .templ files → *_templ.go
generate:
	templ generate ./templates/...

# Full build (generate first)
build: generate
	go build $(LDFLAGS) -o steuerpilot .

# Cross-compile for Windows AMD64
build-windows: generate
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o steuerpilot.exe .

# Run the compiled binary
run: build
	./steuerpilot

# Development: templ watch + air hot reload (in parallel)
dev:
	templ generate --watch &
	air

# All tests (requires generate)
test: generate
	go test ./...

# Tax calculator unit tests only — fast, no network calls
test-calc:
	go test ./internal/tax/... -v -run .

# Remove generated files and binary
clean:
	find . -name "*_templ.go" -delete
	rm -f steuerpilot steuerpilot.exe
	rm -rf tmp/
