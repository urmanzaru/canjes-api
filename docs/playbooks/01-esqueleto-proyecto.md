# 01 · Esqueleto del proyecto

| | |
|---|---|
| **Objetivo** | Crear el módulo Go, la estructura de carpetas de §7.2, el `Makefile`, Postgres local con Docker Compose, la configuración del linter y un "hola mundo" que compile. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1 (jue 1 oct) |
| **Depende de** | 00 |
| **Desbloquea** | 02, 03, 04 |
| **Secciones del SD** | §7.1, §7.2, §7.3 |
| **Tiempo estimado** | 1–2 h + 3 h de estudio |

---

## Notas para ti (sin tecnicismos)

Es como montar los estantes de una bodega antes de recibir la mercancía. Cada carpeta tiene un propósito y **una regla de quién puede hablar con quién** (ver [`ARCHITECTURE.md`](../ARCHITECTURE.md) §3). Hacerlo bien ahora evita que, en una semana, todo esté revuelto.

También se crea el **Makefile**: una lista de "atajos" para que no tengas que recordar comandos largos. En vez de escribir `go test -tags=integration -race -count=1 ./test/integration/...`, escribes `make test-int`.

Y se levanta **Postgres en tu máquina** dentro de Docker: tu propia base de datos de práctica, que puedes borrar y volver a crear sin miedo.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Tour of Go — *Basics* completo (paquetes, imports, funciones, variables, `for`, `if`, `switch`, `defer`) | https://go.dev/tour/basics/1 |
| Qué es un módulo y `go.mod` | https://go.dev/doc/tutorial/create-module |
| Por qué existe `internal/` y `cmd/` | https://go.dev/doc/modules/layout |
| Qué es un Makefile (solo la idea) | Pídele a Claude: "explícame un Makefile en 5 líneas" |
| Docker Compose: qué es un servicio y un volumen | https://docs.docker.com/compose/gettingstarted/ |

---

## Requisitos previos

- [ ] Playbook 00 completo (`go`, `docker`, `golangci-lint`, `sqlc` responden).
- [ ] Repo clonado; estás en la carpeta `canjes-api`.
- [ ] `CLAUDE.md` y `docs/` ya existen (vienen en el repo).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-project-layout` | Estructura `cmd/` + `internal/`, nombre del módulo, `.gitignore`, checklist de inicio. |
| `golang-dependency-management` | `go mod init`, `go get -tool` para fijar versiones de `oapi-codegen` y `goose`. |
| `golang-lint` | Primer `.golangci.yml` razonable. |
| `golang-dependency-injection` | Confirmar la decisión: inyección **manual** en `main.go`, sin contenedores. |

---

## Plan (paso a paso)

### 1. Módulo

```bash
go mod init github.com/urmanzaru/canjes-api
```

> Si el repo se mueve a la organización de la empresa, el módulo cambia a `github.com/<organización>/canjes-api` (SD §7.1). Hazlo **antes** de escribir mucho código.

### 2. Carpetas (vacías por ahora, con un `doc.go` o `.gitkeep`)

```
api/  cmd/api/  cmd/canjesctl/
internal/config/  internal/codes/  internal/service/
internal/httpapi/gen/  internal/httpapi/middleware/
internal/store/queries/  internal/store/db/  internal/notify/
migrations/  seeds/  test/integration/
```

Cada paquete en `internal/` lleva un `doc.go` de una línea explicando su responsabilidad (copiar de la tabla de `ARCHITECTURE.md` §3).

### 3. Herramientas fijadas en `go.mod`

```bash
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go get -tool github.com/pressly/goose/v3/cmd/goose@latest
```

Se usan como `go tool oapi-codegen …` y `go tool goose …`. `sqlc` se usa instalado (playbook 00).

### 4. Programas mínimos

- `cmd/api/main.go`: imprime `canjes-api <versión>` y sale. La versión se inyecta al compilar con `-ldflags "-X main.version=…"` (playbook 13); por defecto `dev`.
- `cmd/canjesctl/main.go`: imprime la ayuda de subcomandos (se implementan en el 06).

### 5. Postgres local — `docker-compose.yml`

```yaml
services:
  db:
    image: postgres:17-alpine        # usa la misma versión mayor que tendrás en producción
    environment:
      POSTGRES_USER: canjes
      POSTGRES_PASSWORD: canjes
      POSTGRES_DB: canjes
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U canjes"]
      interval: 2s
      retries: 15
volumes:
  pgdata:
```

### 6. Variables de entorno — `.env.example` (SD §7.3, ajustado a local)

```dotenv
PORT=8080
DATABASE_URL=postgres://canjes:canjes@localhost:5432/canjes?sslmode=disable
LOG_LEVEL=debug
RATE_LIMIT_REDEEM_PER_MIN=10
RATE_LIMIT_ISSUE_PER_MIN=30
NOTIFY_WEBHOOK_URL=
TRUSTED_PROXY_HOPS=0
```

`cp .env.example .env`. El `.env` **nunca** se sube a git. (`TRUSTED_PROXY_HOPS` se explica en el playbook 10.)

### 7. `.gitignore`

`/bin/`, `.env`, `coverage.*`, `*.test`, `.DS_Store`, `/tmp/`.

> Los archivos **generados** (`internal/httpapi/gen/`, `internal/store/db/`) **sí** se versionan.

### 8. `Makefile`

| Objetivo | Qué hace |
|---|---|
| `make help` | Lista los objetivos |
| `make tools` | Instala/verifica herramientas de generación con versión fija (`sqlc`, `golangci-lint`) |
| `make generate` | `go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml` + `sqlc generate` |
| `make db-up` / `make db-down` / `make db-reset` | Levanta, detiene o borra y recrea el Postgres local |
| `make migrate` | `go run ./cmd/canjesctl migrate up` (antes del 06: `go tool goose -dir migrations postgres "$$DATABASE_URL" up`) |
| `make run` | `go run ./cmd/api` con las variables de `.env` |
| `make test` | `go test -race ./...` |
| `make test-int` | `go test -tags=integration -race -count=1 ./test/integration/...` |
| `make lint` | `golangci-lint run` |
| `make vuln` | `govulncheck ./...` |
| `make build` | Compila `bin/canjes-api` y `bin/canjesctl` |
| `make docker` | Construye la imagen |

El Makefile carga `.env` con `-include .env` + `export` para que `make run` y `make migrate` vean las variables.

### 9. `.golangci.yml` (formato v2)

Linters por defecto + `gosec`, `errorlint`, `bodyclose`, `rowserrcheck`, `noctx`, `contextcheck`, `revive`. **Sin `misspell`**: revisa ortografía en inglés y marca como error todo el texto en español ("comando", "argumentos"). Excluir carpetas generadas (`internal/httpapi/gen`, `internal/store/db`).

### 10. `README.md` del repo

Breve: qué es, enlace a `docs/`, y la sección "Arranque rápido" (`make db-up`, `make migrate`, `make run`).

---

## Cómo saber que quedó bien

```bash
go build ./...              # sin errores
go run ./cmd/api            # imprime: canjes-api dev
make db-up                  # Postgres arriba
docker compose ps           # db … (healthy)
psql "postgres://canjes:canjes@localhost:5432/canjes" -c 'select 1'   # devuelve 1
make lint                   # sin hallazgos
go tool oapi-codegen -version && go tool goose -version
```

- [ ] La estructura coincide con `ARCHITECTURE.md` §4.
- [ ] `.env` existe y **no** aparece en `git status`.
- [ ] `go.mod` lista `oapi-codegen` y `goose` en un bloque `tool`.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `port 5432 already in use` | Ya tienes otro Postgres instalado | Detenlo o cambia el puerto a `"5433:5432"` y ajusta `DATABASE_URL` |
| `go: cannot find main module` | No estás en la carpeta del repo | `cd` a la raíz (donde está `go.mod`) |
| `golangci-lint` dice que su versión de Go es menor que la del proyecto | Se instaló un binario compilado con un Go anterior | `make tools` (lo compila con el Go del proyecto) |
| `make: *** missing separator` | El Makefile usa espacios en vez de tabuladores | Las recetas del Makefile **deben** empezar con TAB |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-project-layout, golang-dependency-management, golang-lint).
Implementa el playbook docs/playbooks/01-esqueleto-proyecto.md. La estructura debe
coincidir con docs/ARCHITECTURE.md §4 y docs/SYSTEM_DESIGN.md §7.2. No implementes
lógica todavía. Al final corre las comprobaciones del playbook y explícame qué es
go.mod, qué hace internal/ y cómo se usa el Makefile.
```

---

## Próximos pasos

- ➡️ **02 — Contrato OpenAPI**, **03 — Base de datos** y **04 — Generador de códigos** (pueden ir en paralelo).
- Recomendado: 02 primero, porque el contrato es lo primero que conviene compartir con quienes integran las apps.
