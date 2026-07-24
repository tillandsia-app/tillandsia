.PHONY: build build-all test lint clean

build:
	go build -o bin/tillandsia ./cmd/tillandsia
	go build -o bin/tillandsia-init ./cmd/tillandsia-init

build-all:
	CGO_ENABLED=0 go build -o bin/tillandsia-linux-amd64 ./cmd/tillandsia
	CGO_ENABLED=0 go build -o bin/tillandsia-init-linux-amd64 ./cmd/tillandsia-init

test:
	go test ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

clean:
	rm -rf bin/