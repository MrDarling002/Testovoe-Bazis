.PHONY: build test test-integration cover lint fmt up down

build:
	go build ./...

test:
	go test ./...

test-integration:
	go test -tags integration ./internal/store/mysql/ -v

cover:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run

fmt:
	gofmt -w .

up:
	docker compose up --build -d

down:
	docker compose down
