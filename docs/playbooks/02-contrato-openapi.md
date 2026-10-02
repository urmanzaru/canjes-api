# 02 · Contrato OpenAPI

| | |
|---|---|
| **Objetivo** | Escribir `api/openapi.yaml` con todos los endpoints de SD §5 y generar el código del servidor con `oapi-codegen` (servidor `std-http`, modo *strict*). |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1 (jue 1 oct) |
| **Depende de** | 01 |
| **Desbloquea** | 05 (y a los equipos de Emprende con Flow y VisorUS POS) |
| **Secciones del SD** | D2, D12, D13, §5 completo |
| **Tiempo estimado** | 2–3 h + 3 h de estudio |

---

## Notas para ti (sin tecnicismos)

El contrato es el **menú del restaurante**: dice qué se puede pedir (endpoints), qué datos hay que dar (peticiones) y qué te van a servir (respuestas y errores). Se escribe **antes** de cocinar.

Dos ventajas enormes:

1. **Las apps pueden empezar ya.** En cuanto el contrato exista, quien programa Emprende con Flow y VisorUS POS sabe exactamente qué enviar y qué recibir, sin esperar a que la API esté lista. **Compártelo hoy mismo.**
2. **El compilador te vigila.** `oapi-codegen` convierte el menú en una "lista de platillos obligatorios" en Go. Si agregas un endpoint al YAML y olvidas programarlo, el proyecto no compila. Imposible que se te pase.

Este archivo es la **fuente de verdad del HTTP**: si quieres cambiar un endpoint, cambias el YAML y regeneras; nunca al revés.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Qué es una API REST, verbos (`GET`, `POST`) y códigos de estado | https://developer.mozilla.org/es/docs/Web/HTTP/Status |
| JSON y `camelCase` | https://gobyexample.com/json |
| Qué es OpenAPI y cómo se lee un YAML de OpenAPI | https://learn.openapis.org/ |
| Tour of Go — *More types* (structs, punteros, slices, maps) | https://go.dev/tour/moretypes/1 |
| Qué es una interfaz en Go (solo la idea) | https://go.dev/tour/methods/9 |
| `oapi-codegen` (README, sección *std-http* y *strict server*) | https://github.com/oapi-codegen/oapi-codegen |

---

## Requisitos previos

- [ ] Playbook 01 completo; `go tool oapi-codegen -version` responde.
- [ ] Leíste SD §5 completo (endpoints, errores, convenciones).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-design-patterns` | Diseño de API: versionado `/v1`, recursos, paginación. |
| `golang-naming` | Nombres de tipos generados claros (`IssueCodeRequest`, `OfferList`). |
| `golang-error-handling` | Modelar el esquema `ApiError` (D13) de forma uniforme. |
| ⚠️ **No** `golang-swagger` | Usa swaggo (código → OpenAPI). Aquí es al revés (ver [`SKILLS.md`](../SKILLS.md) §5). |

> No hay skill para `oapi-codegen`; seguir su README. Lo importante es aplicar los principios de las skills anteriores.

---

## Plan (paso a paso)

### 1. Esqueleto del YAML

- `openapi: 3.0.3` (oapi-codegen soporta 3.0 de forma estable).
- `info`: título `canjes-api`, versión `1.0.0`, descripción con enlace al SD.
- `servers`: `http://localhost:8080` y `https://canjes.<dominio>`.
- `security`: esquema global `ApiKeyAuth` (`type: apiKey`, `in: header`, `name: X-Api-Key`).

### 2. Componentes (`components/schemas`)

| Esquema | Campos (SD §5) |
|---|---|
| `ApiError` | `error` (string, MAYÚSCULAS), `message` (string) — **requeridos** |
| `Benefit` | `type` (string), `value` (number) |
| `TargetApp` | `slug`, `name` |
| `StoreLinks` | `ios`, `android` (opcionales) |
| `HolderStatus` | `activeCode` (`{codeId, last4, expiresAt}` o `null`), `redeemedCount`, `remaining` |
| `Offer` | `offerId`, `slug`, `title`, `description`, `benefits[]`, `costPoints`, `targetApp`, `storeLinks`, `soldOut`, `holder` (nullable) |
| `OfferList` | `campaign {slug, endsAt}`, `data: Offer[]` |
| `IssueCodeRequest` | `offerId` (uuid), `holderRef` (string, `maxLength: 128`) |
| `IssuedCode` | `codeId`, `code`, `codeDisplay`, `last4`, `offer`, `expiresAt`, `reissued` |
| `CodeSummary` | `codeId`, `last4`, `status`, `offer`, `issuedAt`, `redeemedAt`, `expiresAt` |
| `ValidateRequest` / `ValidateResponse` | `code` / `valid`, `offer {title, benefits}`, `expiresAt` |
| `RedeemRequest` | `code`, `email` (format email), `redeemerRef`, `emailConsentAt` (date-time), `consentVersion` |
| `RedeemResponse` | `codeId`, `status`, `offer`, `redeemedAt` |
| `CodeStatus` | enum `ISSUED, REDEEMED, FULFILLED, REVOKED` |
| `AdminCode`, `AdminCodePage` | Campos de SD §5.3 + `{data, total}` |
| `CodeEvent`, `OfferStats`, `FulfillRequest {note}`, `RevokeRequest {reason}` | SD §5.3 |

Reglas: JSON en `camelCase`; fechas `format: date-time` (RFC 3339 UTC); IDs `format: uuid`.

### 3. Rutas (SD §5.2–§5.4)

| Método y ruta | `operationId` | Scope (`x-required-scope`) | Respuestas |
|---|---|---|---|
| `GET /v1/offers?holderRef=` | `listOffers` | `offers:read` | 200, 401, 403 |
| `POST /v1/codes` | `issueCode` | `codes:issue` | 201, 401, 403, 404, 409, 410, 422, 429 |
| `GET /v1/codes?holderRef=` | `listHolderCodes` | `codes:issue` | 200, 401, 403, 422 |
| `POST /v1/codes/validate` | `validateCode` | `codes:redeem` | 200, 401, 403, 404, 409, 410, 422, 429 |
| `POST /v1/codes/redeem` | `redeemCode` | `codes:redeem` | 200, 401, 403, 404, 409, 410, 422, 429 |
| `GET /v1/admin/codes` (`campaign, offer, status, size, offset, format`) | `adminListCodes` | `admin` | 200 (`application/json` y `text/csv`) |
| `GET /v1/admin/codes/{id}/events` | `adminListCodeEvents` | `admin` | 200, 404 |
| `POST /v1/admin/codes/{id}/fulfill` | `adminFulfillCode` | `admin` | 200, 404, 409 |
| `POST /v1/admin/codes/{id}/revoke` | `adminRevokeCode` | `admin` | 200, 404, 409 |
| `GET /v1/admin/stats?campaign=` | `adminStats` | `admin` | 200 |
| `GET /healthz`, `GET /readyz` | `healthz`, `readyz` | **sin** auth (`security: []`) | 200, 503 |

- Todas las respuestas de error usan `$ref: ApiError`. Documenta en `description` los códigos `error` posibles de cada endpoint (tablas de SD §5.2).
- `x-required-scope` es una extensión **documental**: el mapa real ruta → scope vive en Go (playbook 07), y una prueba verifica que coinciden.
- Agrega `example` a peticiones y respuestas copiando los JSON del SD: el contrato servirá de documentación para las apps.

### 4. Configuración de generación — `oapi-codegen.yaml`

```yaml
package: gen
output: internal/httpapi/gen/api.gen.go
generate:
  models: true
  std-http-server: true     # usa net/http estándar (Go 1.22+ ServeMux)
  strict-server: true       # handlers tipados: reciben structs y devuelven structs
  embedded-spec: true       # el YAML queda dentro del binario (útil para validar en pruebas)
```

### 5. Generar y compilar

```bash
make generate     # o: go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml
go build ./...
```

Revisa `internal/httpapi/gen/api.gen.go`: busca `type StrictServerInterface interface` — es la lista de métodos que implementarás en los playbooks 05–11.

### 6. Validar y previsualizar el contrato (opcional)

```bash
npx @redocly/cli lint api/openapi.yaml
npx @redocly/cli preview-docs api/openapi.yaml   # documentación navegable en el navegador
```

### 7. Compartir

Manda `api/openapi.yaml` (o el enlace al archivo en GitHub) a quien integra Emprende con Flow y VisorUS POS, junto con la sección "Integración en las apps" del SD.

---

## Cómo saber que quedó bien

- [ ] `make generate && go build ./...` sin errores.
- [ ] `redocly lint` sin errores (avisos aceptables).
- [ ] `StrictServerInterface` tiene **12 métodos**: los 10 endpoints de `/v1` + `healthz` + `readyz`.
- [ ] Cada error de las tablas de SD §5.2 aparece documentado en el endpoint correcto.
- [ ] Los ejemplos del YAML coinciden con los JSON del SD.
- [ ] `go test ./internal/httpapi/` pasa: `contract_test.go` verifica que el YAML sea válido, que cada ruta `/v1` tenga `x-required-scope` válido y respuestas 401/403/500, y que todo error use `ApiError`.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `oapi-codegen` falla con `unsupported` | Se usó una función de OpenAPI 3.1 | Quedarse en `3.0.3` (`nullable: true` en vez de `type: [x, "null"]`) |
| Tipos con nombres raros (`N200Response…`) | Respuestas sin esquema con nombre | Definir todo en `components/schemas` y referenciar con `$ref` |
| Redocly: `nullable-type-sibling` | Un `$ref` envuelto en `allOf` con `nullable: true` sin `type` | Agregar `type: object` junto al `allOf` |
| `staticcheck`: `GetSwagger is deprecated` | Nombre viejo de la función generada | Usar `gen.GetSpec()` |
| Al editar `api.gen.go` se pierde el cambio | Es generado | Nunca se edita; se cambia el YAML y se regenera |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to. Implementa el playbook docs/playbooks/02-contrato-openapi.md.
Escribe api/openapi.yaml cubriendo exactamente SD §5 (rutas, campos, errores y ejemplos)
y oapi-codegen.yaml para std-http + strict-server. Genera el código y compila.
No inventes endpoints ni campos que no estén en el SD; si algo es ambiguo, pregúntame.
Al final explícame qué es StrictServerInterface y cómo se relaciona con el YAML.
```

---

## Próximos pasos

- ➡️ **03 — Base de datos** (si no lo hiciste en paralelo) y luego **05 — Servidor base**, que implementa `healthz`/`readyz` y deja los demás métodos respondiendo `501`.
- 📨 Compartir el contrato con los equipos de las apps.
- Cada vez que el contrato cambie: actualizar el SD, regenerar y avisar a las apps.
