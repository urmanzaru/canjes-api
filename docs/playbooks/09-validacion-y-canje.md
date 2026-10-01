# 09 · Validación y canje

| | |
|---|---|
| **Objetivo** | Implementar `POST /v1/codes/validate` (sin cambiar nada) y `POST /v1/codes/redeem` (una sola sentencia atómica), con el mapeo exacto de errores de SD §5.2 y la bitácora de intentos. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 3 (sáb 3 oct) |
| **Depende de** | 08 |
| **Desbloquea** | 10, 11 |
| **Secciones del SD** | N3, RF4, RF5, D8, D9, D10, D11, §5.2, §6.3, §6.4, §8 bordes 3–7 |
| **Tiempo estimado** | 4 h + 3 h de estudio |

---

## Notas para ti (sin tecnicismos)

Es el momento de **usar el boleto** en VisorUS POS:

1. **Validar:** la persona pega el código y la app pregunta "¿sirve?". La API revisa sin gastarlo y responde qué promoción trae ("1 mes gratis + 1 hora de asesoría"). Si hay un dígito mal escrito, lo sabe **al instante, sin consultar la base** gracias al dígito de control.
2. La persona **se registra** en VisorUS POS y acepta el aviso de uso del correo.
3. **Canjear:** la app manda el código con el correo. La API lo marca como usado **en un solo movimiento** que no se puede partir a la mitad.

**¿Por qué "un solo movimiento"?** Imagina dos cajeros que intentan cobrar el mismo boleto al mismo segundo. Si cada uno primero *mira* el boleto ("está libre") y luego lo *marca*, los dos podrían ver "libre" y los dos lo marcarían. Con una sola instrucción "márcalo **solo si** sigue libre", la base de datos garantiza que **uno gana y el otro recibe "ya fue canjeado"**. Una prueba lo comprueba con 20 intentos simultáneos.

**Validar no aparta el boleto.** Entre validar y canjear pueden pasar minutos (la persona se está registrando) y alguien más podría usarlo si lo compartió. Por eso el canje vuelve a revisar todo.

**Mensajes para la persona:** cada error trae un texto en español listo para mostrar ("Este código ya fue canjeado.", "Este código venció.", …). Si el código es de otra app, la API responde "no es válido para esta app" **sin revelar que existe**.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Condiciones de carrera (*race conditions*) explicado simple | Pídele a Claude: "explícame una condición de carrera con dos cajeros" |
| `UPDATE … FROM … RETURNING` en Postgres | https://www.postgresql.org/docs/current/sql-update.html |
| Errores de Postgres en pgx (`*pgconn.PgError`, `errors.As`) | https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError |
| `errors.As` vs `errors.Is` | https://go.dev/blog/go1.13-errors |
| Validar correos con `net/mail` | https://pkg.go.dev/net/mail#ParseAddress |
| Tour of Go — *Concurrency* (goroutines, `sync.WaitGroup`) | https://go.dev/tour/concurrency/1 |

---

## Requisitos previos

- [ ] 08 completo y sus pruebas de integración pasando.
- [ ] Consultas `RedeemCode` y `GetCodeForCheckByHash` generadas (03).
- [ ] `codes.Normalize` y `codes.MaskEmail` (04).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-database` | Sentencia atómica, `ErrNoRows`, error `23505` por índice, no usar transacción donde no hace falta. |
| `golang-error-handling` | Árbol de decisión "¿por qué falló?" y mapeo a errores de dominio. |
| `golang-concurrency` | Prueba con 20 goroutines, `sync.WaitGroup`, `-race`. |
| `golang-security` | Respuesta 404 que no revela existencia (D9), correo enmascarado en bitácora y logs, validación de entrada. |
| `golang-testing` | Casos de integración de cada error. |

---

## Plan (paso a paso)

### 1. Comprobación compartida — `internal/service/check.go`

`explain(code db.CodeForCheck, appID, now) error` decide, **en este orden**:

| # | Condición | Error |
|---|---|---|
| 1 | No existe **o** `offer.target_app_id ≠ appID` | `ErrCodeNotFound` → 404 |
| 2 | `status ∈ {REDEEMED, FULFILLED}` | `ErrCodeAlreadyRedeemed` → 409 |
| 3 | `status = REVOKED` | `ErrCodeRevoked` → 410 |
| 4 | `expires_at ≤ now` | `ErrCodeExpired` → 410 |
| 5 | (todo bien) | `nil` |

Se usa en `validate` (como verificación principal) y en `redeem` (para explicar un `UPDATE` sin filas).

### 2. `Validate(ctx, appID, rawCode, ip)` — `validate.go` (SD §6.4)

1. `codes.Normalize(rawCode)`; si falla → `ErrInvalidCodeFormat` **sin tocar la BD** (tampoco se inserta evento; queda en el log y cuenta para el límite de intentos del 10).
2. `GetCodeForCheckByHash(Hash(code))`.
3. `explain(...)`.
4. Evento `VALIDATE_OK` o `VALIDATE_FAILED` con `{"reason": "<CODIGO>"}`; `code_id` nulo si no se encontró (o era de otra app, para no ligar el intento a un código ajeno).
5. Respuesta: `valid: true`, `offer {title, benefits}`, `expiresAt`.

### 3. `Redeem(ctx, appID, input, ip)` — `redeem.go` (SD §6.3)

1. **Validar entrada** → `ErrValidation` (422) si:
   - `email`: tras `TrimSpace` + `ToLower`, `mail.ParseAddress` falla **o** la dirección resultante no es idéntica a la entrada (rechaza `"Nombre <a@b.com>"`), o mide > 254.
   - `emailConsentAt` falta o está en el futuro (más de 5 min de tolerancia).
   - `redeemerRef` vacío o > 128. `consentVersion` vacío o > 64.
2. `codes.Normalize` → `ErrInvalidCodeFormat`.
3. `RedeemCode(...)` — **una sentencia**, sin transacción explícita.
   - **1 fila:** evento `REDEEMED` con `{"email": MaskEmail(email), "redeemerRef": …}`; avisar al `Notifier` en segundo plano (el 10 lo implementa; aquí basta un `Notifier` que no hace nada); responder 200.
   - **Error `23505` en `ux_codes_email_per_offer`:** `ErrEmailAlreadyRedeemed` → 409; evento `REDEEM_FAILED`.
   - **0 filas:** `GetCodeForCheckByHash` → `explain(...)` → error; evento `REDEEM_FAILED` con el motivo.
4. **Nunca** registrar en logs el correo completo ni el código.

### 4. Handlers

`ValidateCode` y `RedeemCode` en `handlers.go`. La IP del cliente sale de un helper del middleware (en el 10 se respeta `TRUSTED_PROXY_HOPS`; por ahora `RemoteAddr`).

### 5. Pruebas de integración — `test/integration/redeem_test.go`

| Caso | Esperado | DoD |
|---|---|---|
| `validate` con un dígito cambiado | 422 `INVALID_CODE_FORMAT`; **cero** consultas a la BD (contar eventos: sin cambios) | ✅ 5 |
| `validate` correcto | 200 con título y beneficios; evento `VALIDATE_OK` | |
| `redeem` correcto con `Persona@Gmail.com` | 200; en BD `redeemed_email = persona@gmail.com`, `email_consent_at` y `consent_version` guardados | ✅ 6 |
| Segundo `redeem` del mismo código | 409 `CODE_ALREADY_REDEEMED` | ✅ 6 |
| **20 goroutines** canjean el mismo código | **exactamente 1** respuesta 200; 19 respuestas 409 | ✅ 7 |
| Código cuya oferta es de otra app destino | 404 `CODE_NOT_FOUND` (validate y redeem) | ✅ 8 |
| Mismo correo canjea dos códigos de la misma oferta | 409 `EMAIL_ALREADY_REDEEMED` | ✅ 9 |
| Código revocado por reemisión | 410 `CODE_REVOKED` | ✅ 3 |
| Código vencido (`expires_at` en el pasado) | 410 `CODE_EXPIRED` | |
| Correo inválido / sin `emailConsentAt` | 422 `VALIDATION_ERROR` | |
| Código pegado con espacios, guiones y `\n` | se acepta | |
| Logs del canje | no contienen el correo completo ni el código | |

---

## Cómo saber que quedó bien

```bash
make test-int          # incluye -race
# A mano, con un código emitido en el playbook 08:
curl -s -X POST -H "X-Api-Key: $DEV_KEY_POS" -H 'Content-Type: application/json' \
  -d '{"code":"<código con espacios>"}' localhost:8080/v1/codes/validate | jq
curl -s -X POST -H "X-Api-Key: $DEV_KEY_POS" -H 'Content-Type: application/json' \
  -d '{"code":"<código>","email":"Persona@Gmail.com","redeemerRef":"pos-user-1","emailConsentAt":"2026-10-03T18:00:00Z","consentVersion":"pos-canje-v1"}' \
  localhost:8080/v1/codes/redeem | jq
```

- [ ] Todas las casillas del DoD marcadas en la tabla pasan.
- [ ] La prueba de 20 goroutines pasa **10 veces seguidas** (`go test -tags=integration -run Concurrent -count=10 ./test/integration/...`).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| En la prueba concurrente ganan 2 | Se hizo `SELECT` y luego `UPDATE` en Go | Usar **solo** `RedeemCode` (SD §6.3), sin leer antes |
| Siempre 404 aunque el código existe | Se compara con el `issuer_app_id` en vez del `target_app_id` | La condición es `o.target_app_id = @app_id` |
| `EMAIL_ALREADY_REDEEMED` nunca aparece | Se compara el nombre de constraint equivocado | `pgErr.ConstraintName == "ux_codes_email_per_offer"` |
| Correo guardado con mayúsculas | Normalización antes de validar | `TrimSpace` + `ToLower` **antes** de guardar |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-database, golang-error-handling, golang-concurrency,
golang-security, golang-testing).
Implementa el playbook docs/playbooks/09-validacion-y-canje.md. RedeemCode debe ser
exactamente la sentencia de SD §6.3, sin SELECT previo. Implementa explain() con el orden
del paso 1 y todas las pruebas del paso 5, incluida la de 20 goroutines.
Al terminar, explícame por qué una sola sentencia UPDATE evita el doble canje y cómo
funciona la prueba concurrente.
```

---

## Próximos pasos

- ➡️ **11 — Admin** (crítico: necesitas ver los canjes) y **10 — Límite de intentos y webhook**.
- 📨 Avisa a quien integra VisorUS POS que `validate` y `redeem` funcionan; comparte la tabla de mensajes de SD §5.2.
