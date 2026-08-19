.PHONY: run build test tidy fmt migrate-up migrate-down sqlc

run:
	go run ./cmd/smartkrishi

build:
	go build -o bin/smartkrishi ./cmd/smartkrishi

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

MIGRATE ?= $(shell go env GOPATH)/bin/migrate

migrate-install:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate-up: migrate-install
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" up

migrate-down: migrate-install
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" down 1

sqlc:
	sqlc generate
