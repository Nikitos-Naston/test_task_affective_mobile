APP=subscription-api

.PHONY: run test docker-up docker-down migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

docker-up:
	docker compose up --build

docker-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "postgres://subscriptions:subscriptions@localhost:5432/subscriptions?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://subscriptions:subscriptions@localhost:5432/subscriptions?sslmode=disable" down
