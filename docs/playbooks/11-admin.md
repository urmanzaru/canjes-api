# 11 · Endpoints de administración

| | |
|---|---|
| **Objetivo** | Implementar los endpoints con scope `admin`: listar/exportar códigos (JSON y CSV), ver la bitácora de un código, marcar entregado (`fulfill`), revocar y estadísticas por oferta. |
| **Prioridad** | 🔴 Crítico (sin esto no sabes quién canjeó) |
| **Día sugerido** | Día 3 (sáb 3 oct) |
| **Depende de** | 09 |
| **Desbloquea** | 12, 16 |
| **Secciones del SD** | N4, RF6, D11, D14, §3.1, §5.1 (paginación), §5.3 |
| **Tiempo estimado** | 3 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Esta es **tu ventanilla de oficina**. Con la key de administración puedes:

- **Ver la lista** de códigos filtrada por campaña, oferta o estado, y **descargarla en CSV** para abrirla en Excel o Google Sheets.
- **Ver la historia** de un código: cuándo se emitió, si alguien intentó validarlo, cuándo se canjeó.
- **Marcar entregado** cuando ya activaste la promoción a esa persona en VisorUS POS (con una nota como "Activado el 10-oct").
- **Revocar** un código que todavía no se usa (por ejemplo, si detectas abuso).
- **Ver el tablero**: por cada oferta, cuántos códigos se emitieron, cuántos están pendientes, canjeados, entregados, revocados y cuántas validaciones fallaron.

Reglas que la API respeta: no se puede "entregar" algo que no se canjeó, y **un canje no se deshace** (no se puede revocar un código ya canjeado; si hay que compensar a alguien, se resuelve fuera de la API).

> **Cuidado con el CSV:** contiene correos de personas. Guárdalo en un lugar seguro y no lo compartas por canales abiertos.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Parámetros de consulta (`?status=REDEEMED&size=50`) | https://developer.mozilla.org/es/docs/Learn/Common_questions/Web_mechanics/What_is_a_URL |
| Paginación con `size`/`offset` | Pídele a Claude un ejemplo con SQL `LIMIT/OFFSET` |
| CSV en Go (`encoding/csv`) | https://pkg.go.dev/encoding/csv |
| *CSV injection* (fórmulas en celdas) | https://owasp.org/www-community/attacks/CSV_Injection |
| Filtros opcionales en sqlc (`sqlc.narg`) | https://docs.sqlc.dev/en/latest/howto/named_parameters.html |

---

## Requisitos previos

- [ ] 09 completo (hay códigos canjeados para probar).
- [ ] Key admin de desarrollo en `.env` (`DEV_KEY_ADMIN`).
- [ ] Consultas de `admin.sql` y `events.sql` generadas (03).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-database` | Filtros opcionales con parámetros, `COUNT` para el total, consultas de agregación, transiciones con `UPDATE … WHERE status = … RETURNING`. |
| `golang-security` | CSV injection, PII (correos) solo en endpoints admin, límites de `size`. |
| `golang-error-handling` | Distinguir "no existe" (404) de "estado no permite la acción" (409). |
| `golang-design-patterns` | Respuesta en *streaming* para el CSV. |
| `golang-testing` | Pruebas de integración por endpoint. |

---

## Plan (paso a paso)

### 1. Listar — `GET /v1/admin/codes`

- Filtros: `campaign` (slug), `offer` (slug o id), `status`; paginación `size` (por defecto 50, máximo 500) y `offset` (≥ 0). Valores fuera de rango → 422.
- `format=json` (por defecto): `{ "data": [...], "total": N }` con: `codeId`, oferta (slug y título), `last4`, `holderRef`, `status`, `issuedAt`, `redeemedAt`, `redeemedEmail`, `redeemerRef`, `fulfilledAt`, `fulfilledNote`, `revokedAt`, `revokeReason`.
- `format=csv`: `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="canjes-<fecha>.csv"`, mismas columnas; ignora `size`/`offset` y exporta todo lo filtrado, escribiendo fila por fila (sin cargar todo en memoria).
- **CSV injection:** cualquier celda que empiece con `=`, `+`, `-`, `@`, tabulador o retorno de carro se antepone con `'`.

### 2. Bitácora — `GET /v1/admin/codes/{id}/events`

Lista cronológica de `code_events` del código (tipo, `detail`, `client_app`, `ip`, fecha). 404 si el código no existe.

### 3. Entregar — `POST /v1/admin/codes/{id}/fulfill` `{ "note": "…" }`

- `FulfillCode`: `UPDATE codes SET status='FULFILLED', fulfilled_at=now(), fulfilled_note=$2 WHERE id=$1 AND status='REDEEMED' RETURNING …`.
- Sin filas: si no existe → 404 `CODE_NOT_FOUND`; si existe en otro estado → 409 `INVALID_STATE` ("Solo se puede entregar un código canjeado.").
- Evento `FULFILLED` con la nota. `note` ≤ 500 caracteres.

### 4. Revocar — `POST /v1/admin/codes/{id}/revoke` `{ "reason": "…" }`

- Solo `ISSUED → REVOKED` (`revoke_reason = 'ADMIN: <reason>'`). Otro estado → 409 `INVALID_STATE` ("Solo se puede revocar un código sin canjear."). Evento `REVOKED`.

### 5. Estadísticas — `GET /v1/admin/stats?campaign=`

Por oferta: `issued` (total emitidos), `active` (`ISSUED` no vencidos), `redeemed`, `fulfilled`, `revoked`, `expired` (`ISSUED` vencidos), `validateFailed` y `redeemFailed` (de `code_events`). Una sola consulta con `COUNT(*) FILTER (WHERE …)`.

### 6. Actualizar el SD

`INVALID_STATE` (409) no está en SD §5.3. Agrégalo a la tabla de errores del SD y al `openapi.yaml`.

### 7. Pruebas de integración — `test/integration/admin_test.go`

| Caso | Esperado | DoD |
|---|---|---|
| `GET /v1/admin/codes?status=REDEEMED&format=csv` | CSV con cabecera y una fila por canje | ✅ 10 |
| `fulfill` de un código `REDEEMED` | 200 y estado `FULFILLED` | ✅ 10 |
| `fulfill` de un código `ISSUED` | 409 `INVALID_STATE` | |
| `revoke` de `ISSUED` | 200; luego `redeem` → 410 | |
| `revoke` de `REDEEMED` | 409 | |
| Cualquier endpoint admin con key ECF o POS | 403 | |
| `size=1000` | 422 | |
| `holderRef` que empieza con `=` en el CSV | celda con `'=` | |
| `stats` | números coinciden con los códigos creados en la prueba | |
| `events` de un código canjeado | `ISSUED`, `VALIDATE_OK`, `REDEEMED` en orden | |

---

## Cómo saber que quedó bien

```bash
make test-int
curl -s -H "X-Api-Key: $DEV_KEY_ADMIN" "localhost:8080/v1/admin/stats?campaign=ecf-prueba" | jq
curl -s -H "X-Api-Key: $DEV_KEY_ADMIN" "localhost:8080/v1/admin/codes?status=REDEEMED" | jq
curl -s -H "X-Api-Key: $DEV_KEY_ADMIN" "localhost:8080/v1/admin/codes?status=REDEEMED&format=csv" -o canjes.csv
curl -s -X POST -H "X-Api-Key: $DEV_KEY_ADMIN" -H 'Content-Type: application/json' \
  -d '{"note":"Activado en POS"}' localhost:8080/v1/admin/codes/<codeId>/fulfill | jq
```

- [ ] `canjes.csv` abre bien en Excel/Sheets (acentos correctos).
- [ ] Las casillas del DoD marcadas en la tabla pasan.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| Acentos rotos en Excel | Excel espera BOM en UTF-8 | Escribir `﻿` al inicio del CSV |
| `total` no cuadra con `data` | `COUNT` con filtros distintos | Usar los mismos parámetros en ambas consultas |
| El CSV tarda o consume mucha memoria | Se armó todo en memoria | Escribir fila por fila con `csv.Writer` y `Flush` |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-database, golang-security, golang-error-handling,
golang-design-patterns, golang-testing).
Implementa el playbook docs/playbooks/11-admin.md según SD §5.3 y §3.1.
Incluye protección contra CSV injection, el error INVALID_STATE (agrégalo al openapi.yaml
y propón el cambio al SD) y todas las pruebas del paso 7.
Al terminar, enséñame los curl que usaré cada día durante el piloto.
```

---

## Próximos pasos

- ➡️ **12 — Pruebas de integración y concurrencia**: verificar el *Definition of Done* completo.
- Estos `curl` serán tu rutina diaria en el piloto (playbook 16). Considera guardarlos en un script `scripts/admin.sh`.
