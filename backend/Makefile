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

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

sqlc:
	sqlc generate
