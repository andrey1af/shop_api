SHELL := /bin/sh

.DEFAULT_GOAL := help

.PHONY: help docker-build up down logs ps lint fmt fmt-check vet test hooks migrate-up \
	proto-gen proto-lint sqlc-gen diagrams \
	prod-build prod-up prod-down prod-logs prod-ps prod-migrate-up

help:
	@printf '%s\n' \
		'Использование: make <цель>' \
		'' \
		'Цели:' \
		'  help             			Показать список доступных команд' \
		'  docker-build     			Собрать Docker-образы (проверка сборки проекта)' \
		'  up               			Собрать и запустить Docker Compose' \
		'  down             			Остановить контейнеры проекта' \
		'  logs             			Следить за логами контейнеров' \
		'  ps               			Показать состояние контейнеров' \
		'  lint             			Проверить Go-код через локальный golangci-lint' \
		'  fmt              			Отформатировать Go-код (gofmt -w)' \
		'  fmt-check        			Проверить форматирование без изменений (gofmt -l)' \
		'  vet              			Проверить Go-код через go vet' \
		'  test             			Прогнать go test ./... во всех сервисах' \
		'  hooks            			Подключить git-хуки из .githooks (pre-push)' \
		'  migrate-up       			Применить все миграции БД' \
		'  proto-gen        			Сгенерировать Go-код из .proto-контрактов (buf generate)' \
		'  proto-lint       			Проверить .proto-контракты (buf lint)' \
		'  sqlc-gen         			Сгенерировать Go-код из SQL-запросов через sqlc' \
		'  diagrams         			Сгенерировать PNG-диаграммы из docs/diagrams/*.puml (PlantUML)' \
		'' \
		'  prod-build       			Собрать Docker-образы в прод-конфигурации' \
		'  prod-up          			Собрать и запустить прод-стек' \
		'  prod-down        			Остановить прод-стек' \
		'  prod-logs        			Следить за логами прод-стека' \
		'  prod-ps          			Показать состояние прод-контейнеров' \
		'  prod-migrate-up  			Применить все миграции БД в проде'


COMPOSE_PROD := docker compose --env-file .env.prod -f compose.prod.yaml

.env:
	cp .env.example .env

.env.prod:
	cp .env.prod.example .env.prod
	@printf '%s\n' '.env.prod создан из .env.prod.example — заполните секреты перед запуском.' >&2
	@exit 1

docker-build: .env
	docker compose build

up: .env
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

lint:
	cd backend/api-service && golangci-lint run --config ../../.golangci.yml

fmt:
	cd backend/api-service && gofmt -w .

fmt-check:
	@files="$$(gofmt -l backend/api-service)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' 'gofmt: следующие файлы не отформатированы (gofmt -w):' "$$files" >&2; \
		exit 1; \
	fi

vet:
	cd backend/api-service && go vet ./...

test:
	cd backend/api-service && go test ./...
	cd backend/auth-service && go test ./...
	cd backend/generator-service && go test ./...
	cd backend/image-service && go test ./...

hooks:
	git config core.hooksPath .githooks

migrate-up: .env
	docker compose run --rm migrate

proto-gen:
	buf generate

proto-lint:
	buf lint

diagrams:
	plantuml -tpng -o ../images docs/diagrams/*.puml

sqlc-gen:
	cd backend/auth-service && sqlc generate
	cd backend/image-service && sqlc generate
	cd backend/api-service && sqlc generate

prod-build: .env.prod
	$(COMPOSE_PROD) build

prod-up: .env.prod
	$(COMPOSE_PROD) up --build -d

prod-down: .env.prod
	$(COMPOSE_PROD) down

prod-logs: .env.prod
	$(COMPOSE_PROD) logs -f

prod-ps: .env.prod
	$(COMPOSE_PROD) ps

prod-migrate-up: .env.prod
	$(COMPOSE_PROD) run --rm migrate
