.PHONY: build test lint vuln clean

build:
	go build -o alaya-tui ./cmd/alaya-tui/

test:
	go test -race -short ./...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

clean:
	rm -f alaya-tui

run:
	go run ./cmd/alaya-tui/
