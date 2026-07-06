.PHONY: all build test lint clean run-matching run-user run-wallet run-settlement run-gateway proto docker-up docker-down migrate

# Build all services
all: build

build:
	go build -o bin/api-gateway ./cmd/api-gateway
	go build -o bin/user-service ./cmd/user-service
	go build -o bin/matching-engine ./cmd/matching-engine
	go build -o bin/wallet-service ./cmd/wallet-service
	go build -o bin/settlement-service ./cmd/settlement-service

# Run individual services (for development)
run-matching:
	go run ./cmd/matching-engine

run-user:
	go run ./cmd/user-service

run-wallet:
	go run ./cmd/wallet-service

run-settlement:
	go run ./cmd/settlement-service

run-gateway:
	go run ./cmd/api-gateway

# Testing
test:
	go test ./... -v -race -count=1

test-unit:
	go test ./internal/... -v -race -count=1

test-matching:
	go test ./internal/matching/... -v -race -count=1

test-integration:
	go test ./test/integration/... -v -count=1

bench:
	go test ./internal/matching/... -bench=. -benchmem

# Code quality
lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

# Protobuf generation
proto:
	protoc --go_out=. --go-grpc_out=. api/proto/*.proto

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build

# Database
migrate:
	psql -h localhost -U exchange -d exchange -f internal/db/postgres/migrations/001_init.sql

# Clean
clean:
	rm -rf bin/
