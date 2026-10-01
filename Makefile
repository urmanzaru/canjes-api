# Atajos de desarrollo de canjes-api. `make help` lista todo.
# Las recetas DEBEN empezar con TAB, no con espacios.

# Carga .env (si existe) y exporta sus variables a los comandos.
-include .env
export

SQLC_VERSION          := v1.31.1
GOLANGCI_LINT_VERSION := v2.14.0
GOBIN                 := $(shell go env GOPATH)/bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
IMAGE   ?= canjes-api

.DEFAULT_GOAL := help
.PHONY: help tools generate db-up db-down db-reset migrate run test test-int lint vuln build docker clean

help: ## Lista los objetivos disponibles
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

tools: ## Instala sqlc y golangci-lint con versión fija (oapi-codegen y goose van por `go tool`)
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@latest

generate: ## Regenera código desde api/openapi.yaml (oapi-codegen) y el SQL (sqlc)
	@if [ -f api/openapi.yaml ]; then go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml; else echo "api/openapi.yaml aún no existe (playbook 02)"; fi
	@if [ -f sqlc.yaml ]; then $(GOBIN)/sqlc generate; else echo "sqlc.yaml aún no existe (playbook 03)"; fi

db-up: ## Levanta Postgres local en Docker y espera a que esté listo
	docker compose up -d --wait db

db-down: ## Detiene Postgres local (conserva los datos)
	docker compose down

db-reset: ## Borra los datos de Postgres local y lo vuelve a levantar
	docker compose down -v
	docker compose up -d --wait db

migrate: ## Aplica las migraciones pendientes (canjesctl desde el playbook 06)
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

run: ## Ejecuta la API en local con las variables de .env
	go run ./cmd/api

test: ## Pruebas unitarias (sin Docker)
	go test -race ./...

test-int: ## Pruebas de integración contra Postgres real (requiere Docker)
	go test -tags=integration -race -count=1 ./test/integration/...

lint: ## Revisión estática con golangci-lint
	$(GOBIN)/golangci-lint run ./...

vuln: ## Busca vulnerabilidades conocidas en dependencias
	$(GOBIN)/govulncheck ./...

build: ## Compila bin/canjes-api y bin/canjesctl
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/canjes-api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/canjesctl ./cmd/canjesctl

docker: ## Construye la imagen Docker linux/amd64 (playbook 13)
	docker buildx build --platform linux/amd64 --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) --load .

clean: ## Borra binarios y reportes de cobertura
	rm -rf bin coverage.*
