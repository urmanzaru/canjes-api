# CLAUDE.md — canjes-api

API en Go + PostgreSQL que **emite** códigos de canje de un solo uso desde una app (Emprende con Flow) y los **canjea** en otra (VisorUS POS).

## Fuente de verdad

- **`docs/SYSTEM_DESIGN.md` manda.** Si una skill, una librería o una sugerencia contradice el system design, gana el system design. Si algo no está definido ahí, pregunta antes de inventar.
- `docs/ARCHITECTURE.md` explica cómo se traduce el diseño a paquetes y capas.
- `docs/playbooks/` es el plan de implementación por etapas. Trabaja **un playbook a la vez**, en el orden de `docs/playbooks/README.md`.
- `api/openapi.yaml` es el contrato HTTP (D2). Se edita el YAML y se regenera; nunca se edita `internal/httpapi/gen/`.
- Las migraciones de `migrations/` son el esquema. El SQL de `internal/store/queries/` se escribe a mano; nunca se edita `internal/store/db/` (lo genera `sqlc`).

## Stack (no cambiar sin actualizar el system design)

Go (estable más reciente) · PostgreSQL · `net/http` estándar · `pgx/v5` · `sqlc` · `goose` · `oapi-codegen` (servidor `std-http`) · `log/slog` · `golang.org/x/time/rate` · `testcontainers-go` · Docker (distroless).

No usar: ORMs (GORM, ent), frameworks web (gin, echo, fiber), `swaggo/swag` (el contrato es primero, no se genera desde anotaciones), contenedores de DI (wire, dig, fx, samber/do): la inyección es manual por constructores en `cmd/api/main.go`.

## Reglas que no se rompen

1. **Dependencias entre capas:** `httpapi → service → store`. `internal/codes` no depende de nadie. Los handlers no escriben SQL; el servicio no conoce HTTP.
2. **Nunca** guardar ni registrar en logs el código completo, la API key ni el correo completo. En BD solo hashes SHA-256 + `last4`; en logs y `code_events`, valores enmascarados.
3. **El canje es una sola sentencia SQL atómica** (`UPDATE … WHERE status = 'ISSUED' … RETURNING`, §6.3). No reemplazar por lógica en Go con mutex.
4. **La emisión es una transacción con `SELECT … FOR UPDATE` sobre la oferta** (§6.2).
5. Las reglas de unicidad viven en índices parciales de Postgres (§4), no solo en Go.
6. Errores HTTP siempre con el formato `{ "error": "CODIGO_EN_MAYUSCULAS", "message": "Texto en español" }` (D13).
7. Todo acceso a BD recibe `context.Context`.
8. Cada cambio de comportamiento lleva su prueba; los casos del *Definition of Done* (§0.2) son pruebas de integración.

## Comandos (cuando existan en el Makefile)

```bash
make tools       # instala herramientas de generación
make generate    # oapi-codegen + sqlc
make db-up       # levanta Postgres local en Docker
make migrate     # aplica migraciones
make run         # levanta la API en local
make test        # pruebas unitarias
make test-int    # pruebas de integración (requiere Docker)
make lint        # golangci-lint
make docker      # construye la imagen
```

## Go development

Before any Go coding, review, debugging, troubleshooting, or setup task, load the `samber/cc-skills-golang@golang-how-to` skill first — it routes to whichever other Go skills the task needs.

## Required Go skills

The following Go skills from `samber/cc-skills-golang` MUST always be applied when working on this project. Load them at the start of every Go-related task, regardless of whether the user explicitly mentions them.

- `samber/cc-skills-golang@golang-error-handling`
- `samber/cc-skills-golang@golang-security`
- `samber/cc-skills-golang@golang-database`
- `samber/cc-skills-golang@golang-testing`
- `samber/cc-skills-golang@golang-safety`
- `samber/cc-skills-golang@golang-naming`
- `samber/cc-skills-golang@golang-code-style`

Cuando una skill sugiera algo distinto al system design (por ejemplo, `golang-database` recomienda `golang-migrate`; aquí se usa `goose`), aplica el **principio** de la skill con la **herramienta** del system design. Detalle en `docs/SKILLS.md`.

## Sobre quién mantiene este repo

Uriel está aprendiendo Go mientras construye esta API. Al terminar cada tarea, explica en 3–5 líneas y en lenguaje sencillo **qué se hizo y por qué**, y qué concepto de Go conviene repasar (ver `docs/APRENDIZAJE.md`).
