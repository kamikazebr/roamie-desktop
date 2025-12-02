# Makefile for Roamie VPN

.PHONY: all build test test-coverage test-html clean

# Build targets
all: build

build:
	./scripts/build.sh

# Test targets
test:
	go test -race ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	rm -f roamie roamie-server coverage.out coverage.html

# Development helpers
.PHONY: dev-setup migrate

dev-setup:
	./scripts/docker-dev.sh setup

migrate:
	./scripts/migrate.sh
