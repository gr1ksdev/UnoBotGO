.PHONY: local db up down logs simulator

# ──────────────────────────────────────────
# 3 modos de rodar o bot
# ──────────────────────────────────────────
# local  → postgres local + app local
# db     → postgres docker  + app local
# up     → postgres docker  + app docker

# ── Modo 1: tudo local ────────────────────
# Requer: PostgreSQL rodando na máquina
# Usa o DATABASE_URL padrão do config.go
local:
	go run .

# ── Modo 2: banco no Docker, app local ────
# Sobe só o PostgreSQL no Docker e aguarda ready
db:
	docker compose up -d --wait db
	DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable' go run .

# ── Modo 3: tudo no Docker ────────────────
# Sobe app + PostgreSQL
up:
	docker compose up -d

# ── Utilitários ───────────────────────────
down:
	docker compose down

logs:
	docker compose logs -f

# Simulador local da engine V2 (quantidade e modo são solicitados no terminal)
simulator:
	go run ./cmd/simulator
