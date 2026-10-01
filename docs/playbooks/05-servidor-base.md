# 05 · Servidor base: config, logs y salud — ¿Cómo ejecutarlo?

| | |
|---|---|
| **Objetivo** | Tener la API corriendo en local: leer configuración, logs JSON con `request_id`, middlewares base (request-id, logging, recover), formato de errores D13, `/healthz`, `/readyz` y apagado ordenado. Los endpoints de negocio responden `501` hasta que se implementen. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 2 (vie 2 oct) |
| **Depende de** | 02 (código generado), 03 (pool de BD) |
| **Desbloquea** | 07, 13 |
| **Secciones del SD** | D13, §1.2 (operación), §5.1, §5.4, §7.3, §9 (observabilidad) |
| **Tiempo estimado** | 3 h + 4 h de estudio |

---

## Notas para ti (sin tecnicismos)

Hasta ahora había piezas sueltas; aquí **se enciende el local por primera vez**. Aún no se venden boletos, pero:

- **La puerta abre y cierra bien:** el programa arranca, se conecta a la base y, cuando lo apagas, termina de atender a quien estaba dentro antes de cerrar (apagado ordenado).
- **Hay un portero que anota todo:** cada visita recibe un número (`request_id`) y queda en el registro (logs) con hora, qué pidió, cómo le fue y cuánto tardó. Nunca se anotan datos sensibles (códigos, keys, correos completos).
- **Hay un guardia de emergencias:** si algo se rompe dentro, la API responde "error interno" en lugar de caerse.
- **Hay un letrero de "abierto":** `/healthz` (el programa vive) y `/readyz` (y además puede hablar con la base). Las plataformas en la nube los usan para saber si reiniciar la API.
- **Todos los errores hablan igual:** `{ "error": "CODIGO", "message": "Texto en español" }`, que las apps pueden mostrar tal cual.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Servidor HTTP en Go: `http.Handler`, `HandlerFunc`, `ServeMux` | https://gobyexample.com/http-server · https://pkg.go.dev/net/http |
| Middleware (función que envuelve a otra) | Pídele a Claude: "explícame un middleware en Go con un ejemplo de 15 líneas" |
| `context.Context` | https://gobyexample.com/context · https://go.dev/blog/context |
| Logs estructurados con `log/slog` | https://go.dev/blog/slog |
| Variables de entorno en Go | https://gobyexample.com/environment-variables |
| Señales y apagado ordenado (`signal.NotifyContext`, `Server.Shutdown`) | https://gobyexample.com/signals |
| Tour of Go — *Methods and interfaces* | https://go.dev/tour/methods/1 |

---

## Requisitos previos

- [ ] 02 completo: existe `internal/httpapi/gen/api.gen.go`.
- [ ] 03 completo: migraciones aplicadas y `internal/store/store.go` con `NewPool`.
- [ ] `.env` creado desde `.env.example`.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-design-patterns` | Constructores, sin `init()` ni globales, apagado ordenado, timeouts del servidor. |
| `golang-observability` | `slog` JSON, campos estándar por petición, qué **no** registrar. |
| `golang-error-handling` | Middleware de logging/recover, una sola traducción de errores a HTTP. |
| `golang-context` | `request_id` y app cliente dentro del `context`; claves de contexto privadas. |
| `golang-dependency-injection` | Inyección manual en `cmd/api/main.go`. |

---

## Plan (paso a paso)

### 1. Configuración — `internal/config/config.go`

- `type Config struct { Port int; DatabaseURL string; LogLevel slog.Level; RateLimitRedeemPerMin, RateLimitIssuePerMin int; NotifyWebhookURL string; TrustedProxyHops int }`
- `Load() (Config, error)`: lee con `os.Getenv`, aplica valores por defecto de SD §7.3, valida (`DATABASE_URL` obligatorio, puertos y números positivos) y devuelve **todos** los errores juntos (`errors.Join`).
- Prueba unitaria con `t.Setenv`.

### 2. Logger

`slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))`. Se pasa por constructor; no se usa `slog.Default()` dentro de los paquetes.

### 3. Errores HTTP — `internal/httpapi/errors.go`

- `writeError(w, status, code, message)` escribe `ApiError` con `Content-Type: application/json`.
- `fromDomainError(err) (status int, body gen.ApiError)`: tabla de `ARCHITECTURE.md` §8 con `errors.Is`. Cualquier error desconocido → `500 INTERNAL_ERROR` con mensaje genérico, y se registra el error real con el `request_id`.
- Los errores de dominio (`ErrOfferNotFound`, …) se declaran ya en `internal/service/errors.go`, aunque el servicio llegue después.
- Prueba unitaria: cada error de dominio → estado y código esperados.

### 4. Middlewares — `internal/httpapi/middleware/`

| Archivo | Qué hace |
|---|---|
| `requestid.go` | Si llega `X-Request-Id` válido (≤ 64 caracteres, `[A-Za-z0-9-_]`), lo usa; si no, genera un UUID. Lo guarda en el `context` y lo devuelve en la respuesta. |
| `logging.go` | Al terminar cada petición, un log `info` con: `request_id`, `method`, `route` (patrón, no la URL con parámetros), `status`, `duration_ms`, `client_app` (si hay), `error_code` (si hubo). **Nunca** cuerpo de la petición, `X-Api-Key`, códigos ni correos. |
| `recover.go` | Atrapa *panics*, registra `error` con el *stack*, responde `500 INTERNAL_ERROR`. |

Orden (de afuera hacia adentro): `requestid → logging → recover → (auth, rate-limit en 07 y 10) → handler`.

### 5. Handlers y servidor — `internal/httpapi/`

- `handlers.go`: `type Handlers struct{ … }` que implementa `gen.StrictServerInterface`. Por ahora cada método devuelve `501 NOT_IMPLEMENTED` excepto:
  - `Healthz` → `200 {"status":"ok"}`.
  - `Readyz` → `pool.Ping(ctx)` con timeout de 2 s → `200` o `503`.
- `server.go`: `New(deps) http.Handler` que crea el `ServeMux`, registra las rutas generadas (`gen.HandlerFromMux(gen.NewStrictHandlerWithOptions(…), mux)`) y envuelve con los middlewares. En `StrictHTTPServerOptions`, `RequestErrorHandlerFunc` traduce JSON inválido a `422 VALIDATION_ERROR` y `ResponseErrorHandlerFunc` usa `fromDomainError`.
- Compilar con `var _ gen.StrictServerInterface = (*Handlers)(nil)` (comprobación en tiempo de compilación).

### 6. Programa — `cmd/api/main.go`

1. `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
2. `config.Load()` → logger → `store.NewPool` → `httpapi.New(...)`.
3. `http.Server{Addr, Handler, ReadHeaderTimeout: 5s, ReadTimeout: 10s, WriteTimeout: 15s, IdleTimeout: 60s}`.
4. `ListenAndServe` en una goroutine; esperar `ctx.Done()`; `Shutdown` con 10 s; `pool.Close()`.
5. Log `info` al arrancar (puerto, versión, nivel) y al terminar.
6. `main` solo llama a `run(ctx) error`; si hay error, lo registra y sale con código 1.

### 7. Ejecutar (la respuesta a "¿cómo ejecutarlo?")

```bash
make db-up        # 1. Postgres local
make migrate      # 2. tablas al día
make run          # 3. API en http://localhost:8080
```

En otra terminal:

```bash
curl -i localhost:8080/healthz
curl -i localhost:8080/readyz
curl -i -H 'X-Request-Id: prueba-1' localhost:8080/v1/offers   # 501 por ahora (401 desde el playbook 07)
```

Detener: `Ctrl+C` (debe verse el log de apagado ordenado).

---

## Cómo saber que quedó bien

- [ ] `make run` arranca y muestra un log JSON de inicio.
- [ ] `/healthz` → 200. `/readyz` → 200; con `make db-down` → 503; con `make db-up` vuelve a 200 sin reiniciar la API.
- [ ] La respuesta trae `X-Request-Id`; si lo envías, se respeta.
- [ ] Cada petición produce **una** línea de log JSON con los campos del paso 4.
- [ ] Un endpoint de negocio responde `501` con `{"error":"NOT_IMPLEMENTED","message":"…"}`.
- [ ] Un *panic* forzado en una prueba responde 500 y no tumba el proceso.
- [ ] `Ctrl+C` termina limpio en < 10 s.
- [ ] Sin `DATABASE_URL`, el programa sale con un mensaje claro.
- [ ] `go test ./internal/config ./internal/httpapi/...` pasa.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `make run` no ve las variables | `.env` no se carga | Revisa `-include .env` y `export` en el Makefile |
| Logs con la URL completa incluyendo `holderRef` | Se registró `r.URL.String()` | Registrar `r.Pattern` (Go 1.23+) |
| `/readyz` tarda mucho cuando la BD está caída | Sin timeout | `context.WithTimeout(ctx, 2*time.Second)` en el `Ping` |
| Respuestas de error con texto plano | Se usó `http.Error` | Usar siempre `writeError` |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-design-patterns, golang-observability, golang-error-handling,
golang-context, golang-dependency-injection).
Implementa el playbook docs/playbooks/05-servidor-base.md siguiendo docs/ARCHITECTURE.md §3, §8 y §11.
Los endpoints de negocio deben responder 501 por ahora. Escribe pruebas para config,
errors.go y los middlewares (con httptest). Al terminar, ejecuta la API, muéstrame los
curl del paso 7 y explícame qué es un middleware y cómo viaja el request_id en el context.
```

---

## Próximos pasos

- ➡️ **06 — CLI** (si no está) y **07 — Autenticación**, que se inserta en la cadena de middlewares.
- ➡️ **13 — Construir** ya se puede hacer en cualquier momento a partir de aquí (útil para ensayar el despliegue temprano).
