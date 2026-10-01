# canjes-api

API en Go + PostgreSQL para **emitir** códigos de canje de un solo uso desde una app (Emprende con Flow) y **canjearlos** en otra (VisorUS POS), guardando con qué correo se canjeó cada uno.

> **Estado:** diseño listo, implementación en curso por playbooks. Primer uso: piloto de Emprende con Flow, 7–20 de octubre de 2026.

## Documentación

| Documento | Para qué |
|---|---|
| [`docs/SYSTEM_DESIGN.md`](docs/SYSTEM_DESIGN.md) | **Fuente de verdad**: requisitos, decisiones, modelo de datos, contrato y reglas |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Cómo se traduce el diseño a paquetes, capas y flujos |
| [`docs/playbooks/`](docs/playbooks/README.md) | Plan de implementación en 17 pasos, con orden y dependencias |
| [`docs/SKILLS.md`](docs/SKILLS.md) | Skills de Claude Code para Go (`samber/cc-skills-golang`) y cómo aplican aquí |
| [`docs/APRENDIZAJE.md`](docs/APRENDIZAJE.md) | Ruta para aprender Go mientras se construye la API |
| [`CLAUDE.md`](CLAUDE.md) | Reglas del proyecto para Claude Code |

## Preguntas frecuentes

| Pregunta | Respuesta corta | Detalle |
|---|---|---|
| **¿Cómo instalarlo en local?** | Instala Go, Docker, `sqlc` y `golangci-lint`; clona el repo; `cp .env.example .env` | [Playbook 00](docs/playbooks/00-preparar-entorno.md) · [01](docs/playbooks/01-esqueleto-proyecto.md) |
| **¿Cómo ejecutarlo?** | `make db-up && make migrate && make run` → `http://localhost:8080/healthz` | [Playbook 05](docs/playbooks/05-servidor-base.md) |
| **¿Cómo construir?** | `make build` (binarios) · `make docker` (imagen distroless `linux/amd64`) | [Playbook 13](docs/playbooks/13-construir.md) |
| **¿Cómo lanzarlo a producción?** | Imagen al registro → `canjesctl migrate up` → desplegar → semilla y keys → simulacro | [Playbook 15](docs/playbooks/15-produccion.md) |

## Arranque rápido (cuando el código exista)

```bash
cp .env.example .env
make db-up          # Postgres local en Docker
make migrate        # crea las tablas
go run ./cmd/canjesctl seed seeds/dev.sql
make run            # API en http://localhost:8080
curl localhost:8080/healthz
```

## Stack

Go · PostgreSQL · `net/http` · `pgx/v5` · `sqlc` · `goose` · `oapi-codegen` · `log/slog` · Docker (distroless).
