# 03 · Base de datos: migraciones y consultas

| | |
|---|---|
| **Objetivo** | Crear la migración inicial (copia exacta de SD §4) con `goose`, escribir a mano las consultas SQL de §6 y generar su código Go con `sqlc` + `pgx/v5`. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1 (jue 1 oct) |
| **Depende de** | 01 |
| **Desbloquea** | 05, 06 |
| **Secciones del SD** | D3, D7, D8, D10, D11, §4, §6.2, §6.3 |
| **Tiempo estimado** | 2–3 h + 4 h de estudio |

---

## Notas para ti (sin tecnicismos)

La base de datos es **la libreta oficial** de la taquilla. Aquí se decide qué columnas tiene cada hoja (tablas) y, sobre todo, **qué reglas no se pueden romper nunca**:

- "Una persona no puede tener dos códigos vigentes para la misma oferta."
- "Un correo no puede canjear dos veces la misma oferta."

Esas reglas se ponen **en la propia base de datos** (índices únicos). Aunque el programa tuviera un error, Postgres se niega a guardar algo que las rompa. Es como un notario que no firma un documento duplicado aunque se lo pidas.

Una **migración** es una receta numerada para crear o cambiar tablas. Se aplica una vez, en orden, en tu máquina y en producción, para que ambas tengan exactamente la misma estructura.

Las **consultas** se escriben en SQL normal (lo que puedes leer y probar en `psql`), y `sqlc` las convierte en funciones Go. Así las dos reglas críticas del sistema (emitir con cupo y canjear una sola vez) se ven tal cual son, sin magia escondida.

> **Importante:** el esquema ya lo diseñaste tú en el SD. Aquí **se copia, no se rediseña**. Si durante la implementación algo no cuadra, se corrige primero el SD.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| SQL: `CREATE TABLE`, tipos (`uuid`, `text`, `timestamptz`, `jsonb`), `PRIMARY KEY`, `REFERENCES` | https://www.postgresql.org/docs/current/tutorial-sql.html |
| Índices y **índices parciales** (`… WHERE status = 'ISSUED'`) | https://www.postgresql.org/docs/current/indexes-partial.html |
| Transacciones (`BEGIN`, `COMMIT`, `ROLLBACK`) | https://www.postgresql.org/docs/current/tutorial-transactions.html |
| `SELECT … FOR UPDATE` (bloqueo de filas) | https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS |
| `UPDATE … RETURNING` | https://www.postgresql.org/docs/current/dml-returning.html |
| Qué hace `sqlc` (tutorial de PostgreSQL) | https://docs.sqlc.dev/en/latest/tutorials/getting-started-postgresql.html |
| Migraciones con `goose` | https://pressly.github.io/goose/ |
| Tutorial oficial de Go con BD (para la idea de `Query`, `Scan`, `error`) | https://go.dev/doc/tutorial/database-access |

---

## Requisitos previos

- [ ] Playbook 01 completo; `make db-up` deja Postgres "healthy".
- [ ] `sqlc version` y `go tool goose -version` responden.
- [ ] Leíste SD §4 y §6.2–§6.3.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-database` | Parámetros (nunca concatenar), `context` en todo, `pgx` vs `database/sql`, pool de conexiones, transacciones y `FOR UPDATE`, manejo de "no encontrado". |
| `golang-error-handling` | Distinguir `pgx.ErrNoRows` y errores de Postgres (`*pgconn.PgError`, código `23505`). |
| `golang-security` | Inyección SQL (sqlc usa parámetros), privilegios mínimos del usuario de BD. |

> Ajustes respecto a la skill (ver [`SKILLS.md`](../SKILLS.md) §5): se usa **goose** (no golang-migrate) y el esquema viene del SD (no se rediseña).

---

## Plan (paso a paso)

### 1. Migración inicial — `migrations/00001_init.sql`

Copia **literal** del bloque SQL de SD §4 (incluye `-- +goose Up` / `-- +goose Down`).

### 2. Migraciones dentro del binario — `migrations/embed.go`

```go
// Package migrations incrusta los archivos SQL para que canjesctl pueda aplicarlos sin leer disco.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

### 3. Aplicar en local

```bash
make db-up
go tool goose -dir migrations postgres "$DATABASE_URL" up
go tool goose -dir migrations postgres "$DATABASE_URL" status
psql "$DATABASE_URL" -c '\dt'          # 6 tablas
psql "$DATABASE_URL" -c '\d codes'     # columnas e índices ux_codes_one_active, ux_codes_email_per_offer
```

### 4. Configuración de `sqlc` — `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: migrations            # sqlc entiende el formato de goose
    queries: internal/store/queries
    gen:
      go:
        package: db
        out: internal/store/db
        sql_package: pgx/v5
        emit_pointers_for_null_types: true   # columnas NULL → *string, *time.Time
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
          - db_type: uuid
            nullable: true
            go_type:
              import: github.com/google/uuid
              type: UUID
              pointer: true
          - db_type: timestamptz
            go_type: time.Time
          - db_type: timestamptz
            nullable: true
            go_type:
              type: time.Time
              pointer: true
```

### 5. Consultas a mano — `internal/store/queries/`

| Archivo | Consultas (`-- name: X :one/:many/:exec`) | Uso |
|---|---|---|
| `apps.sql` | `CreateClientApp`, `GetClientAppBySlug`, `ListClientApps` | CLI |
| `keys.sql` | `CreateAPIKey`, `FindActiveKeyByHash` (une con `client_apps.active` y `revoked_at IS NULL`), `RevokeAPIKeyByPrefix`, `ListAPIKeys` | CLI, auth |
| `campaigns.sql` | `CreateCampaign`, `SetCampaignStatus`, `GetActiveCampaignForIssuer` | CLI, ofertas |
| `offers.sql` | `CreateOffer`, `ListActiveOffersForCampaign`, `CountUsedQuotaByOffer` (para `soldOut`), `HolderOfferStatus` | CLI, ofertas |
| `codes_issue.sql` | `LockOfferForIssue` (SD §6.2, literal), `CountRedeemedByHolder`, `RevokeActiveCodeForHolder`, `CountUsedQuota`, `InsertCode`, `ListCodesByHolder` | emisión |
| `codes_redeem.sql` | `RedeemCode` (SD §6.3, literal), `GetCodeForCheckByHash` (une con `offers` para `target_app_id`, título y beneficios) | validar, canjear |
| `admin.sql` | `AdminListCodes` + `AdminCountCodes` (filtros opcionales con `sqlc.narg`), `FulfillCode`, `AdminRevokeCode`, `StatsByCampaign` | admin |
| `events.sql` | `InsertCodeEvent`, `ListEventsByCode` | todos |

Reglas al escribirlas:

- **Siempre parámetros** (`$1`, `@nombre`, `sqlc.narg('status')`), nunca texto concatenado.
- `RedeemCode` y `LockOfferForIssue` se copian **exactamente** del SD; tienen comentarios explicando por qué son así.
- `code_events` solo tiene `INSERT` y `SELECT` (D11: bitácora que solo crece).

### 6. Generar

```bash
sqlc generate      # o make generate
go mod tidy        # baja pgx/v5 y google/uuid
go build ./...
```

### 7. Acceso a la BD — `internal/store/store.go`

- `NewPool(ctx, databaseURL) (*pgxpool.Pool, error)`: parsea la URL, fija `MaxConns` (10 basta para el piloto), `MaxConnIdleTime`, y hace `Ping`.
- `InTx(ctx, pool, fn func(q *db.Queries, tx pgx.Tx) error) error`: abre transacción, ejecuta `fn`, hace `Commit` o `Rollback` (con `defer`). Lo usa la emisión.
- Helper `IsUniqueViolation(err, constraint string) bool` usando `errors.As(err, &*pgconn.PgError)` y `Code == "23505"`.

### 8. Experimento para aprender (10 minutos, en `psql`)

```sql
-- Inserta datos mínimos a mano y comprueba que el notario funciona:
-- 1) dos códigos ISSUED para el mismo (offer_id, holder_ref) → debe fallar con ux_codes_one_active
-- 2) dos códigos con el mismo redeemed_email para la misma oferta → debe fallar con ux_codes_email_per_offer
```

Pídele a Claude que te dé los `INSERT` exactos. Ver el error con tus ojos ayuda a entender D7 y D10.

---

## Cómo saber que quedó bien

- [ ] `goose status` muestra `00001_init.sql` aplicado.
- [ ] `\d codes` muestra los dos índices únicos parciales y los dos índices normales.
- [ ] `sqlc generate` sin errores; `internal/store/db/` contiene `models.go` y un `.go` por archivo de consultas.
- [ ] `go build ./...` compila.
- [ ] `go tool goose -dir migrations postgres "$DATABASE_URL" down` y luego `up` funcionan (la migración es reversible en local).
- [ ] El experimento del paso 8 produjo los dos errores esperados.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `sqlc`: `column "x" does not exist` | La consulta no coincide con el esquema | Revisa nombres contra `00001_init.sql` |
| Tipos `pgtype.UUID` / `pgtype.Timestamptz` en el código generado | Faltan los `overrides` | Revisa `sqlc.yaml` paso 4 |
| `function gen_random_uuid() does not exist` | Postgres muy viejo o falta la extensión | La migración ya crea `pgcrypto`; usa Postgres ≥ 13 |
| `goose: no migrations found` | Ruta incorrecta | `-dir migrations` desde la raíz del repo |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-database, golang-error-handling, golang-security).
Implementa el playbook docs/playbooks/03-base-de-datos.md.
Copia el SQL de SD §4 EXACTAMENTE en migrations/00001_init.sql; no cambies el esquema.
Escribe las consultas de la tabla del paso 5; LockOfferForIssue y RedeemCode deben ser
idénticas a SD §6.2 y §6.3. Configura sqlc con pgx/v5 y los overrides de uuid/timestamptz.
Al final guíame en el experimento del paso 8 y explícame qué es un índice único parcial.
```

---

## Próximos pasos

- ➡️ **04 — Generador de códigos** (si no está) y **05 — Servidor base** (usa `NewPool` para `/readyz`).
- ➡️ **06 — CLI** usa `migrations.FS` y las consultas de `apps`, `keys`, `campaigns`, `offers`.
- Cualquier cambio futuro de esquema = **nueva** migración (`00002_…sql`), nunca editar la `00001` una vez en producción.
