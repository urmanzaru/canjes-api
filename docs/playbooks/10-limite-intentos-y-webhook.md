# 10 · Límite de intentos y webhook de avisos

| | |
|---|---|
| **Objetivo** | (a) Limitar intentos: 10 **fallidos** por minuto por (key, IP) en `validate`/`redeem` y 30 por minuto por key en `POST /v1/codes` → 429. (b) Aviso opcional a Slack/Google Chat en cada canje, en segundo plano. |
| **Prioridad** | 🟡 Recomendado antes del piloto (puede desplegarse entre el 5 y el 7 de octubre) |
| **Día sugerido** | Día 3 (sáb 3 oct) |
| **Depende de** | 09 |
| **Desbloquea** | 12 |
| **Secciones del SD** | RF8, §6.6, §6.7, §8 borde 9, §9 |
| **Tiempo estimado** | 3 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

**Límite de intentos.** Adivinar un código de 40 dígitos es imposible en la práctica, así que esto no es contra "hackers adivinando". Es contra **errores**: una app con un bug que reintenta sin parar, o alguien probando códigos a mano. Tras 10 intentos fallidos en un minuto desde la misma app y la misma conexión, la API responde **"Demasiados intentos. Espera un minuto."** (429). Los intentos **exitosos no cuentan**.

**Webhook.** Si quieres, cada canje manda un mensaje a un canal de Slack o Google Chat:
`✅ Canje: VisorUS POS · 1 mes + asesoría · ***1930 · p***@gmail.com`.
Es solo **un aviso**: si Slack falla, el canje igual queda registrado y la persona no espera. La fuente de verdad siempre es `GET /v1/admin/codes`.

**Limitación aceptada:** el contador vive en la memoria del servidor. Si algún día hay dos servidores, cada uno cuenta por su lado (SD §8 borde 9). Para el piloto hay uno solo.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| *Rate limiting* y el algoritmo de "cubeta de fichas" (*token bucket*) | https://pkg.go.dev/golang.org/x/time/rate |
| Goroutines en segundo plano y `context.WithoutCancel` | https://pkg.go.dev/context#WithoutCancel |
| Cliente HTTP con timeout | https://gobyexample.com/http-client |
| `sync.WaitGroup` para esperar tareas al apagar | https://gobyexample.com/waitgroups |
| Webhooks entrantes de Slack / Google Chat | https://api.slack.com/messaging/webhooks · https://developers.google.com/workspace/chat/quickstart/webhooks |
| `X-Forwarded-For` y por qué no se puede confiar ciegamente | https://developer.mozilla.org/es/docs/Web/HTTP/Headers/X-Forwarded-For |

---

## Requisitos previos

- [ ] 09 completo.
- [ ] (Webhook) Una URL de webhook de prueba de Slack o Google Chat, o https://webhook.site para pruebas.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-concurrency` | Mapa de limitadores protegido, limpieza periódica sin fugas de goroutines, `WaitGroup` en el notificador. |
| `golang-context` | `context.WithoutCancel` + timeout de 3 s para el webhook (que no muera cuando termina la petición). |
| `golang-security` | IP real detrás de proxies, no filtrar datos en el aviso (correo y código enmascarados), validar la URL del webhook. |
| `golang-design-patterns` | Middleware, timeouts en toda llamada externa, apagado ordenado. |
| `golang-testing` | Reloj inyectable, `httptest.Server` como webhook falso, `goleak` opcional. |

---

## Plan (paso a paso)

### 1. IP del cliente — `middleware/clientip.go`

- `TRUSTED_PROXY_HOPS` (agregado a la configuración en el 05; **no está en SD §7.3**, documéntalo en el SD): número de proxies de confianza delante de la API.
  - `0` (local, VPS sin proxy): usar `RemoteAddr`.
  - `N > 0`: tomar de `X-Forwarded-For` la entrada en la posición *N-ésima desde la derecha* (las de la izquierda las puede inventar el cliente).
- El valor correcto para la plataforma elegida se fija en el playbook 15.

### 2. Limitador — `middleware/ratelimit.go`

```go
type limiterSet struct {
    mu       sync.Mutex
    byKey    map[string]*entry   // entry{lim *rate.Limiter, lastSeen time.Time}
    perMin   int
}
```

| Endpoint | Clave | Regla |
|---|---|---|
| `POST /v1/codes/validate`, `POST /v1/codes/redeem` | `keyPrefix + "|" + ip` | Antes del handler: si `lim.Tokens() < 1` → 429. Después: si la respuesta fue **fallida** (404, 409, 410, 422) → consumir una ficha (`lim.Allow()`). |
| `POST /v1/codes` | `keyPrefix` | Cada petición consume una ficha (`lim.Allow()`); si no hay → 429. |

- Límite configurable: `RATE_LIMIT_REDEEM_PER_MIN` (10) y `RATE_LIMIT_ISSUE_PER_MIN` (30): `rate.NewLimiter(rate.Every(time.Minute/time.Duration(n)), n)`.
- Para saber el estado de la respuesta, envolver el `ResponseWriter` (el de logging ya lo hace; reutilizarlo).
- Limpieza: una goroutine con `time.Ticker` cada 5 min borra entradas sin uso > 10 min; se detiene con el `context` del servidor (sin fugas).
- 429: `{"error":"TOO_MANY_ATTEMPTS","message":"Demasiados intentos. Espera un minuto."}` + encabezado `Retry-After: 60`.
- Orden en la cadena: `requestid → logging → recover → auth → ratelimit → handler`.

### 3. Notificador — `internal/notify/webhook.go`

```go
type Notifier interface{ Send(ctx context.Context, text string) }

type Webhook struct { url string; client *http.Client; wg sync.WaitGroup; log *slog.Logger }
func (w *Webhook) Send(ctx context.Context, text string)   // lanza goroutine y regresa al instante
func (w *Webhook) Close(ctx context.Context) error          // espera envíos en curso al apagar
type Noop struct{}                                          // si NOTIFY_WEBHOOK_URL está vacío
```

- Cuerpo JSON `{"text": "..."}` (lo aceptan Slack y Google Chat).
- `http.Client{Timeout: 3 * time.Second}`, **sin reintentos**; si falla, log `warn` sin el contenido sensible.
- La goroutine usa `context.WithoutCancel(ctx)` con su propio timeout: la petición del canje ya respondió, pero el aviso debe terminar.
- Mensaje (SD §6.7): `✅ Canje: <app destino> · <título corto> · ***<last4> · <correo enmascarado>`.
- `main.go`: `Close` del notificador después de `server.Shutdown`.
- Validar al arrancar que la URL sea `https://`.

### 4. Pruebas

| Caso | Esperado | DoD |
|---|---|---|
| 10 `validate` fallidos (dígito cambiado) desde la misma key e IP | todos 422 | |
| El 11.º en el mismo minuto | 429 `TOO_MANY_ATTEMPTS` | ✅ 11 |
| Otra IP con la misma key | no está limitada | |
| 30 `validate` **exitosos** seguidos | ninguno 429 | |
| 31 emisiones en un minuto con la misma key | la 31.ª → 429 | |
| Tras avanzar el reloj 1 min | vuelve a permitir | |
| Canje con webhook falso (`httptest.Server`) | llega el texto con `***last4` y correo enmascarado | |
| Webhook que tarda 10 s | el canje responde en < 100 ms; el envío se corta a los 3 s | |
| Sin `NOTIFY_WEBHOOK_URL` | no se intenta enviar nada | |
| `X-Forwarded-For` con `TRUSTED_PROXY_HOPS=1` | se usa la IP correcta, no la inventada | |

---

## Cómo saber que quedó bien

```bash
go test ./internal/httpapi/middleware/... ./internal/notify/... -race
make test-int
for i in $(seq 1 11); do
  curl -s -o /dev/null -w "%{http_code}\n" -X POST -H "X-Api-Key: $DEV_KEY_POS" \
    -H 'Content-Type: application/json' -d '{"code":"1234"}' localhost:8080/v1/codes/validate
done    # diez 422 y un 429
```

- [ ] La secuencia del bucle termina en `429`.
- [ ] Con `NOTIFY_WEBHOOK_URL` apuntando a webhook.site, cada canje aparece allí en segundos.
- [ ] `Ctrl+C` con un envío en curso espera a que termine (o a los 3 s).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| En local todo cuenta como la misma IP | Correcto: todo viene de `127.0.0.1` | Para probar "otra IP" usar `TRUSTED_PROXY_HOPS=1` y `X-Forwarded-For` |
| Los aciertos también bloquean | Se consumió ficha antes de saber el resultado | Revisar `Tokens()` antes y consumir solo si falló |
| El webhook nunca llega | Se usó el `ctx` de la petición (se cancela al responder) | `context.WithoutCancel` |
| Fuga de goroutines en pruebas | El limpiador no se detiene | Detenerlo con el `context` del servidor |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-concurrency, golang-context, golang-security,
golang-design-patterns, golang-testing).
Implementa el playbook docs/playbooks/10-limite-intentos-y-webhook.md siguiendo SD §6.6
y §6.7: solo los intentos fallidos cuentan en validate/redeem, todos cuentan en emisión.
Incluye TRUSTED_PROXY_HOPS para la IP real y el notificador con WaitGroup y timeout de 3 s.
Al terminar, explícame cómo funciona el token bucket y por qué el webhook usa
context.WithoutCancel.
```

---

## Próximos pasos

- ➡️ **11 — Admin** (si no está) y **12 — Pruebas de integración**.
- 📝 Agrega `TRUSTED_PROXY_HOPS` a SD §7.3.
- Decisión A5 (canal del webhook): si no hay canal aún, deja `NOTIFY_WEBHOOK_URL` vacío; todo funciona igual.
