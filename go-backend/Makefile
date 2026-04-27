.PHONY: build run clean test

BINARY_NAME=clippy-backend

build:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME) clippy.db

test:
	go test ./...

deps:
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build  - Build the binary"
	@echo "  run    - Build and run the application"
	@echo "  clean  - Remove binary and database"
	@echo "  test   - Run tests"
	@echo "  deps   - Tidy dependencies"
