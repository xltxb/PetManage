.PHONY: test test-cover lint build dev

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

lint:
	go vet ./...

build:
	go build -o bin/server ./cmd/server/

dev:
	go run ./cmd/server/main.go

clean:
	rm -rf bin/ coverage.out
