main_package_path = ./cmd/api
binary_name = main

.PHONY: dev run build migrate generate-sqlc generate-swagger docker-build docker-up docker-down

## live reload
dev:
	go run github.com/cosmtrek/air@v1.43.0 \
			--build.cmd "make build" --build.bin "/tmp/bin/${binary_name}" --build.delay "100" \
			--build.exclude_dir "" \
			--build.include_ext "go, tpl, tmpl, html, css, scss, js, ts, sql, jpeg, jpg, gif, png, bmp, svg, webp, ico" \
			--misc.clean_on_exit "true"

run:
	go run cmd/api/main.go

build:
	go build -o=/tmp/bin/${binary_name} ${main_package_path}

generate-sqlc:
	sqlc generate

generate-swagger:
	swag init -g cmd/api/main.go -o ./docs

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

all: generate-sqlc generate-swagger run

docker-all:
	docker-compose up --build

migrate-up:
	migrate -path ./internal/db/migrations -database "postgres://postgres:postgres@localhost:5432/data-distributor?sslmode=disable" -verbose up

migrate-down:
	migrate -path ./internal/db/migrations -database "postgres://postgres:postgres@localhost:5432/data-distributor?sslmode=disable" -verbose down