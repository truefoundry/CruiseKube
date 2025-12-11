.PHONY: help test

help:
	@echo "Available targets:"
	@echo "  test              - Run tests"
	@echo "  help              - Show this help message"

test:
	@echo "Running tests..."
	@go test ./...
