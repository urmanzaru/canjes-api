# 08 · Ofertas y emisión de códigos

| | |
|---|---|
| **Objetivo** | Implementar `GET /v1/offers`, `POST /v1/codes` (transacción con bloqueo, revocación del vigente, límite por persona y cupo) y `GET /v1/codes`. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 2 (vie 2 oct) |
| **Depende de** | 04 (generador), 07 (app cliente en el `context`) |
| **Desbloquea** | 09 |
| **Secciones del SD** | N1, N2, RF1–RF3, D6, D7, D11, D15, §5.2, §6.2, §8 bordes 1, 2 y 8 |
| **Tiempo estimado** | 4 h + 3 h de estudio |

---

## Notas para ti (sin tecnicismos)

Aquí la taquilla **empieza a vender boletos**.

- **Ver ofertas** (`GET /v1/offers`): Emprende con Flow pregunta "¿qué promociones hay y cómo va esta persona en cada una?". La respuesta dice si la persona ya tiene un código pendiente, si ya canjeó y si la oferta se agotó.
- **Emitir** (`POST /v1/codes`): la app pide un boleto para una persona y una oferta. La API revisa, en este orden: que la oferta exista y sea de esa app, que la campaña esté en fechas, que la persona no haya alcanzado su límite, que quede cupo… y entonces imprime el boleto.
- **Si la app vuelve a pedir el boleto** (se perdió la señal, la persona reinstaló), la API **anula el anterior y entrega uno nuevo**. Así nunca hay dos boletos válidos para lo mismo, y reintentar es siempre seguro.
- **Mis códigos** (`GET /v1/codes`): lista los boletos de la persona **sin mostrar el número completo** (solo los últimos 4), para que la app pueda reconstruir "Mis canjes".

**La fila ordenada:** cuando dos personas piden el último boleto de una oferta al mismo tiempo, la base de datos las pone en fila (bloqueo `FOR UPDATE`) para que no se vendan 21 boletos de un cupo de 20. Con el volumen del piloto, la espera es de milisegundos.

> **Regla para Emprende con Flow (recuérdasela a quien integra):** descontar los puntos en el teléfono **solo después** de recibir `201`. Si falla la red, no se descuenta nada y se reintenta.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Transacciones en Postgres y en `pgx` (`Begin`, `Commit`, `Rollback`, `defer`) | https://pkg.go.dev/github.com/jackc/pgx/v5#hdr-Transactions |
| `SELECT … FOR UPDATE` | https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS |
| *Savepoints* (transacciones anidadas) | https://www.postgresql.org/docs/current/sql-savepoint.html |
| Errores centinela y `errors.Is` | https://go.dev/blog/go1.13-errors |
| Structs y métodos (el `Service`) | https://go.dev/tour/methods/1 |
| Códigos HTTP 201, 404, 409, 410, 422 | https://developer.mozilla.org/es/docs/Web/HTTP/Status |

---

## Requisitos previos

- [ ] 07 completo: los handlers reciben el `Client` (app que llama).
- [ ] Consultas de `offers.sql`, `campaigns.sql`, `codes_issue.sql`, `events.sql` generadas (03).
- [ ] `seeds/dev.sql` cargada (campaña de prueba activa hoy).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-database` | Transacción, `FOR UPDATE`, `ErrNoRows`, *savepoint* para reintentar el `INSERT`, `context` en todo. |
| `golang-error-handling` | Errores de dominio, envolver con `%w`, una sola traducción a HTTP. |
| `golang-structs-interfaces` | Diseño de `service.Service`, entradas/salidas del servicio separadas de los tipos HTTP generados. |
| `golang-security` | No registrar el código; el código completo solo en la respuesta 201. |
| `golang-testing` | Pruebas de integración de cada regla (contra Postgres real). |

---

## Plan (paso a paso)

### 1. Servicio — `internal/service/service.go`

```go
type Service struct {
    pool   *pgxpool.Pool
    q      *db.Queries
    log    *slog.Logger
    notify Notifier          // se usa en 09; interfaz con Send(ctx, msg)
    now    func() time.Time  // inyectable para pruebas de fechas
}
```

Tipos propios de entrada/salida (`IssueInput`, `IssuedCode`, `OfferView`, …). El handler traduce entre `gen.*` y estos tipos.

### 2. `ListOffers(ctx, appID, holderRef *string)` — `offers.go`

1. `GetActiveCampaignForIssuer(appID)` → si no hay campaña activa: `200` con `data: []` (y `campaign: null`).
2. `ListActiveOffersForCampaign` ordenadas por `sort_order`.
3. Por oferta: `soldOut = total_quota != nil && usados ≥ total_quota` (usados = `ISSUED`+`REDEEMED`+`FULFILLED`).
4. Si viene `holderRef`: `activeCode` (código `ISSUED` y no vencido: `codeId`, `last4`, `expiresAt`), `redeemedCount` (`REDEEMED`+`FULFILLED`), `remaining = max_per_holder − redeemedCount` (mínimo 0). Si no viene: `holder: null`.

> **Decisión a confirmar:** la respuesta tiene **una** `campaign`. Se asume una sola campaña `ACTIVE` por app emisora (el CLI del 06 lo impide). Si en el futuro hay varias simultáneas, el contrato deberá cambiar a una lista (`/v2` o campo nuevo).

### 3. `Issue(ctx, appID, offerID, holderRef)` — `issue.go` (SD §6.2, paso a paso)

| Paso | Acción | Error |
|---|---|---|
| 0 | Validar `holderRef`: no vacío, ≤ 128 | `ErrValidation` → 422 |
| 1 | `BEGIN` (`store.InTx`) | |
| 2 | `LockOfferForIssue(offerID)`; si no hay fila o `issuer_app_id ≠ appID` | `ErrOfferNotFound` → 404 |
| 3 | Campaña `ACTIVE` y `starts_at ≤ now < ends_at` | `ErrCampaignNotActive` → 410 |
| 4 | `CountRedeemedByHolder ≥ max_per_holder` | `ErrHolderLimitReached` → 409 |
| 5 | `RevokeActiveCodeForHolder` (`REVOKED`, `revoke_reason='REISSUED'`) → si revocó, evento `REVOKED` y `reissued = true` | |
| 6 | Si `total_quota` no es nulo: `CountUsedQuota ≥ total_quota` | `ErrOfferSoldOut` → 409 |
| 7 | `Generate` → `Hash`, `Last4` → `InsertCode` con `expires_at = redeem_until`; evento `ISSUED` | |
| 8 | `COMMIT` → devolver código completo + `Display` | |

**Reintento por colisión (paso 7):** en Postgres, un error dentro de una transacción la deja inservible. Por eso el `INSERT` se hace dentro de un *savepoint* (`tx.Begin(ctx)` en pgx crea uno): si choca con `codes_code_hash_key` (`23505`), se deshace solo el *savepoint*, se genera otro código y se reintenta (máximo 3). En la práctica nunca ocurre (10^39 combinaciones), pero el SD lo exige.

**Bitácora:** los eventos se insertan **en la misma transacción** (si la emisión se deshace, el evento también). `detail` nunca lleva el código; `ip` viene del handler.

### 4. `ListHolderCodes(ctx, appID, holderRef)` — `issue.go`

`ListCodesByHolder(issuer_app_id, holder_ref)` ordenado por `issued_at DESC`, con datos básicos de la oferta. Sin código completo. `holderRef` obligatorio → si falta, 422.

### 5. Handlers — `internal/httpapi/handlers.go`

`ListOffers`, `IssueCode` (responde **201**), `ListHolderCodes`: leen `ClientFrom(ctx)`, llaman al servicio y mapean tipos. Los errores van por `fromDomainError`.

### 6. Pruebas de integración — `test/integration/issue_test.go`

(La infraestructura de testcontainers se arma aquí si aún no existe; ver 12 §1.)

| Caso | Esperado | DoD |
|---|---|---|
| `GET /v1/offers` con key ECF | 200 con las ofertas de la campaña | ✅ 1 |
| `GET /v1/offers` con key POS | 403 | ✅ 1 |
| `POST /v1/codes` | 201, código de 40 dígitos con Luhn válido | ✅ 2 |
| En la BD tras emitir | existe `code_hash` y `code_last4`; **no** existe el código en ninguna columna | ✅ 2 |
| Emitir dos veces mismo `(offer, holder)` | segundo 201 con `reissued: true`; el primero queda `REVOKED` | ✅ 3 |
| Canjear el primer código (cuando exista el 09) | 410 `CODE_REVOKED` | ✅ 3 |
| Con un canje hecho, emitir otra vez | 409 `HOLDER_LIMIT_REACHED` | ✅ 4 |
| Oferta de otra app emisora | 404 `OFFER_NOT_FOUND` | |
| Campaña fuera de fechas | 410 `CAMPAIGN_NOT_ACTIVE` | |
| Cupo 2, tres personas | tercera 409 `OFFER_SOLD_OUT`; reemitir a la primera **no** agota | |
| `holderRef` de 129 caracteres | 422 | |
| `GET /v1/codes?holderRef=` | lista sin campo `code` | |
| `GET /v1/offers?holderRef=` tras emitir | `holder.activeCode.last4` coincide | |

---

## Cómo saber que quedó bien

```bash
make test-int
make run
curl -s -H "X-Api-Key: $DEV_KEY_ECF" "localhost:8080/v1/offers?holderRef=persona-1" | jq
OFFER=<offerId de la respuesta anterior>
curl -s -X POST -H "X-Api-Key: $DEV_KEY_ECF" -H 'Content-Type: application/json' \
  -d "{\"offerId\":\"$OFFER\",\"holderRef\":\"persona-1\"}" localhost:8080/v1/codes | jq
psql "$DATABASE_URL" -c "select status, code_last4, revoke_reason from codes order by issued_at"
psql "$DATABASE_URL" -c "select type, detail from code_events order by id"
```

- [ ] Todas las pruebas de la tabla pasan.
- [ ] Repetir el `POST` muestra `reissued: true` y en la BD el anterior aparece `REVOKED / REISSUED`.
- [ ] `grep` del código completo en los logs de la API no encuentra nada.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `current transaction is aborted` al reintentar | Se reintentó sin *savepoint* | Usar `tx.Begin(ctx)` (savepoint) alrededor del `INSERT` |
| `duplicate key … ux_codes_one_active` | Se insertó antes de revocar | Respetar el orden del paso 5 → 7 |
| Se pasan del cupo en pruebas concurrentes | Falta `FOR UPDATE` o se leyó fuera de la transacción | Todas las lecturas del paso 2–6 con la `tx` |
| `reissued` siempre `false` | No se usó `RETURNING id` en la revocación | Revisar `RevokeActiveCodeForHolder` |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-database, golang-error-handling, golang-structs-interfaces,
golang-security, golang-testing).
Implementa el playbook docs/playbooks/08-ofertas-y-emision.md. La emisión debe seguir
SD §6.2 paso a paso dentro de una transacción, con savepoint para el reintento por
colisión. Escribe las pruebas de integración de la tabla del paso 6 con testcontainers.
Al terminar, explícame la transacción de Issue línea por línea y por qué FOR UPDATE
evita vender más boletos que el cupo.
```

---

## Próximos pasos

- ➡️ **09 — Validación y canje**: completa el ciclo de vida del código.
- 📨 Avisa a quien integra Emprende con Flow que `GET /v1/offers` y `POST /v1/codes` ya funcionan en local (y en cuanto despliegues, en el ambiente compartido).
