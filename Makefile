.PHONY: help dev dev-back dev-web web-build preview build run start test test-race vet check clean simulator local db up down logs seed-miniapp clean-miniapp-seed

BIN_DIR := bin
BIN_NAME := $(BIN_DIR)/unobotgo

help:
	@echo "UnoBotGO — Targets disponíveis:"
	@echo "  make dev         Supervisiona backend Go (--dev) e Vite simultaneamente"
	@echo "  make dev-back    Executa backend Go em modo dev (--dev)"
	@echo "  make dev-web     Inicia servidor Vite dev com proxy"
	@echo "  make web-build   Gera o bundle do frontend em web/dist"
	@echo "  make preview     Compila frontend e executa o binário servindo dist embutido"
	@echo "  make build       Compila frontend e binário Go para $(BIN_NAME)"
	@echo "  make run         Executa o binário compilado $(BIN_NAME)"
	@echo "  make start       Build completo e execução imediata"
	@echo "  make test        Executa testes unitários Go e testes do frontend"
	@echo "  make test-race   Executa testes Go com race detector"
	@echo "  make vet         Executa go vet"
	@echo "  make check       Validação completa de qualidade (Go, frontend, gates, diff)"
	@echo "  make clean       Limpa binários e dist gerado (preserva .keep e .env)"
	@echo "  make simulator   Executa simulador de partidas V2"
	@echo "  make local       (Legado V1) Executa go run ."
	@echo "  make db          Sobe PostgreSQL via docker compose"
	@echo "  make up          Sobe app + PostgreSQL via docker compose"
	@echo "  make down        Derruba containers docker compose"
	@echo "  make logs        Exibe logs docker compose"
	@echo "  make seed-miniapp        Popula banco local com fixtures de homologação do Mini App"
	@echo "  make clean-miniapp-seed  Remove exclusivamente as fixtures de homologação do Mini App"

dev:
	node scripts/dev.mjs

dev-back:
	go run ./cmd/bot --dev

dev-web:
	npm --prefix web run dev

web-build:
	npm --prefix web ci
	npm --prefix web run typecheck
	npm --prefix web run build

preview: web-build build
	$(BIN_NAME)

build: web-build
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_NAME) ./cmd/bot

run:
	$(BIN_NAME)

start: build run

test:
	go test -count=1 ./...
	npm --prefix web run test

test-race:
	CGO_ENABLED=1 go test -race ./...

vet:
	go vet ./...

check:
	@if [ -z "$$TEST_DATABASE_URL" ]; then \
		echo "ERRO: TEST_DATABASE_URL obrigatório para executar make check"; \
		exit 1; \
	fi
	npm --prefix web ci
	npm --prefix web run lint
	npm --prefix web run typecheck
	npm --prefix web run test
	npm --prefix web run build
	go test -count=1 ./...
	go vet ./...
	go build ./...
	go test -tags debugcards ./...
	go vet -tags debugcards ./...
	go build -tags debugcards ./...
	go test -count=1 -tags integration ./internal/storage/postgres/... ./internal/app/...
	git diff --check
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_NAME) ./cmd/bot

clean:
	rm -f $(BIN_NAME)
	find web/dist -mindepth 1 ! -name '.keep' -delete 2>/dev/null || true

simulator:
	go run ./cmd/simulator

local:
	go run .

db:
	docker compose up -d --wait db
	DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable' go run ./cmd/bot --dev

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

seed-miniapp:
	@APP_ENV=development go run ./cmd/devseed miniapp

clean-miniapp-seed:
	@APP_ENV=development go run ./cmd/devseed clean-miniapp
