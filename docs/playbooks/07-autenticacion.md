# 07 · Autenticación por API key y scopes

| | |
|---|---|
| **Objetivo** | Middleware que lee `X-Api-Key`, la busca por hash (con caché de 60 s), verifica que la key y su app estén activas y que tenga el **scope** del endpoint. Sin key → 401; sin scope → 403. La app cliente queda disponible en el `context`. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 2 (vie 2 oct) |
| **Depende de** | 05 (cadena de middlewares), 06 (keys para probar) |
| **Desbloquea** | 08 |
| **Secciones del SD** | D4, §0.1 (actores y scopes), §5.1, §6.5, §8 borde 8, §9 seguridad |
| **Tiempo estimado** | 2–3 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Cada app trae su **gafete** (API key). El guardia de la puerta:

1. Revisa que traigas gafete → si no, **401** ("¿quién eres?").
2. Compara la huella del gafete con su lista → si no está o fue dado de baja, **401**.
3. Revisa a qué zonas puede entrar ese gafete (scopes) → si no es la zona correcta, **403** ("sé quién eres, pero aquí no puedes entrar").

| App | Puede | No puede |
|---|---|---|
| Emprende con Flow | ver ofertas, emitir códigos, ver sus códigos | validar, canjear, administrar |
| VisorUS POS | validar y canjear | ver ofertas, emitir, administrar |
| Panel interno (tú) | administrar | lo demás |

Así, si alguien saca la key de Emprende con Flow de la app (es posible: va dentro del teléfono), **no puede canjear** ni ver correos; lo peor que puede hacer es emitir códigos dentro del cupo, y tú lo verías en las estadísticas.

**Caché de 60 s:** para no preguntar a la base en cada visita, el guardia recuerda los gafetes un minuto. Consecuencia práctica: **cuando revocas una key, deja de funcionar en máximo 60 segundos**, no al instante.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Encabezados HTTP y diferencia 401 vs 403 | https://developer.mozilla.org/es/docs/Web/HTTP/Status/401 · …/403 |
| Closures (funciones que devuelven funciones) | https://gobyexample.com/closures |
| Interfaces pequeñas y *fakes* para pruebas | https://go.dev/tour/methods/9 |
| `sync.Mutex` y por qué un `map` compartido necesita protección | https://gobyexample.com/mutexes |
| Valores en `context` con claves privadas | https://pkg.go.dev/context#WithValue |
| `httptest` para probar handlers | https://pkg.go.dev/net/http/httptest |

---

## Requisitos previos

- [ ] 05 completo (cadena de middlewares y `writeError`).
- [ ] 06 completo: tienes en `.env` `DEV_KEY_ECF`, `DEV_KEY_POS`, `DEV_KEY_ADMIN`.
- [ ] Consulta `FindActiveKeyByHash` generada (03).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-security` | Manejo de credenciales, no registrar la key, mensajes de error que no filtran información, revocación. |
| `golang-design-patterns` | Middleware por composición; interfaz `KeyStore` pequeña; decorador de caché. |
| `golang-context` | Guardar la app cliente en el `context` con clave no exportada; funciones `WithClient` / `ClientFrom`. |
| `golang-concurrency` | Caché segura entre goroutines (`sync.Mutex` o `sync.RWMutex`). |
| `golang-testing` | Pruebas de tabla con `httptest` y un `KeyStore` falso. |

---

## Plan (paso a paso)

### 1. Tipos — `internal/httpapi/middleware/auth.go`

```go
type Client struct {
    AppID     uuid.UUID
    AppSlug   string
    KeyPrefix string
    Scopes    []string
}

type KeyStore interface {
    FindActiveKey(ctx context.Context, keyHash string) (Client, error) // ErrKeyNotFound si no existe/revocada/app inactiva
}
```

### 2. Implementación de `KeyStore`

- `storeKeyStore` (en `internal/store` o `httpapi`): llama a `FindActiveKeyByHash` (une `api_keys` con `client_apps`, `revoked_at IS NULL AND client_apps.active`).
- `cachedKeyStore`: envuelve otra `KeyStore`; `map[hash]entry{client, expiresAt}` protegido por `sync.Mutex`; TTL 60 s. **Solo cachea aciertos** (si cachea fallos, una key recién creada tardaría en funcionar). Limpieza simple: al insertar, si el mapa pasa de N entradas, borrar las vencidas.

### 3. Middleware `RequireScope(store KeyStore, scope string)`

Como SD §6.5, con estos detalles:

- Rechazo rápido si la key no empieza con `ck_live_` o no mide 40 → 401 sin consultar BD.
- Errores: `401 UNAUTHORIZED "Falta la API key."` / `"API key inválida."`; `403 FORBIDDEN "Esta app no tiene permiso para esta operación."`.
- Si pasa: `WithClient(ctx, client)` y agrega `client_app` al log de la petición.
- Error inesperado de BD → 500 (no 401: no queremos ocultar una caída como "key inválida").

### 4. Mapa ruta → scope — `internal/httpapi/server.go`

| Patrón | Scope |
|---|---|
| `GET /v1/offers` | `offers:read` |
| `POST /v1/codes`, `GET /v1/codes` | `codes:issue` |
| `POST /v1/codes/validate`, `POST /v1/codes/redeem` | `codes:redeem` |
| `/v1/admin/*` | `admin` |
| `/healthz`, `/readyz` | ninguno |

Implementación sencilla: un `map[string]string` de `"MÉTODO /ruta"` → scope, consultado por un middleware de autenticación general antes del handler generado. **Una ruta sin entrada en el mapa responde 403** (seguro por defecto). Prueba: cada operación del `openapi.yaml` con `x-required-scope` tiene la misma entrada en el mapa.

### 5. Usar la app cliente en los handlers

`middleware.ClientFrom(ctx)` devuelve el `Client`; los handlers lo pasan al servicio como `AppID`. El servicio **nunca** lee encabezados.

### 6. Pruebas (`auth_test.go`, con `httptest` y `fakeKeyStore`)

| Caso | Esperado |
|---|---|
| Sin encabezado | 401 `UNAUTHORIZED` |
| Key con formato inválido | 401, `fakeKeyStore` **no** se llamó |
| Key desconocida o revocada | 401 |
| Key válida sin el scope | 403 `FORBIDDEN` |
| Key válida con el scope | 200 y el handler ve el `Client` en el `context` |
| Error de BD | 500 |
| Caché: dos peticiones en < 60 s | el store real se consulta 1 vez |
| Caché: tras 60 s (reloj inyectable) | se vuelve a consultar |
| Ruta no mapeada | 403 |

---

## Cómo saber que quedó bien

```bash
go test ./internal/httpapi/... -race
make run
curl -i localhost:8080/v1/offers                                       # 401
curl -i -H "X-Api-Key: ck_live_inventada0000000000000000000000" localhost:8080/v1/offers   # 401
curl -i -H "X-Api-Key: $DEV_KEY_POS" localhost:8080/v1/offers          # 403  ← casilla del DoD
curl -i -H "X-Api-Key: $DEV_KEY_ECF" localhost:8080/v1/offers          # 501 (aún no implementado) en vez de 401/403
```

- [ ] Las cuatro respuestas coinciden.
- [ ] `go run ./cmd/canjesctl keys revoke --prefix <prefijo>` → en ≤ 60 s esa key da 401.
- [ ] En los logs aparece `client_app` y **nunca** la key.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| Key revocada sigue funcionando | Caché de 60 s | Es lo esperado; espera un minuto (documentado en SD §6.5) |
| Key nueva tarda en funcionar | Se cachearon fallos | Cachear solo aciertos |
| `go test -race` reporta *data race* en la caché | `map` sin `Mutex` | Proteger lecturas y escrituras |
| Todas las rutas dan 403 | Clave del mapa no coincide (p. ej. `/v1/offers/`) | Usar exactamente el patrón del contrato |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-security, golang-design-patterns, golang-context,
golang-concurrency, golang-testing).
Implementa el playbook docs/playbooks/07-autenticacion.md, partiendo de SD §6.5.
Incluye la caché de 60 s como decorador de KeyStore con reloj inyectable, el mapa
ruta→scope seguro por defecto y todas las pruebas de la tabla del paso 6.
Después explícame por qué 401 y 403 son distintos y cómo funciona la caché.
```

---

## Próximos pasos

- ➡️ **08 — Ofertas y emisión**: el primer endpoint con lógica de negocio.
- Recuerda: la key de Emprende con Flow viajará **dentro de la app**. La solución de fondo (JWT de la persona o emisión servidor a servidor) está en SD §10; para el piloto se mitiga con cupo y vigilancia (playbook 16).
