ifneq ($(wildcard .env),)
    include .env
    export
endif

.PHONY: run test migrate

run:
	docker compose up -d

migrate:
	migrate -path ./migrations -database "$(DATABASE_URL)" up
