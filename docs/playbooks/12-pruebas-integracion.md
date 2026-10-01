# 12 · Pruebas de integración, concurrencia y contrato

| | |
|---|---|
| **Objetivo** | Consolidar la batería de pruebas contra **Postgres real** (testcontainers), con pruebas de concurrencia, validación de respuestas contra `openapi.yaml` y un mapa que demuestre cada casilla del *Definition of Done*. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 4 (dom 4 oct), mañana |
| **Depende de** | 08, 09, 10, 11 |
| **Desbloquea** | 14, 15 |
| **Secciones del SD** | §0.2 (DoD), §1.2, §7.4 |
| **Tiempo estimado** | 3 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Las pruebas son **el examen que la API debe aprobar antes de salir a la calle**. Hay tres tipos:

- **Unitarias:** revisan piezas sueltas (el generador de códigos, el traductor de errores). Rápidas, sin base de datos.
- **De integración:** levantan un Postgres de verdad en una caja de Docker, arrancan la API completa y le hacen peticiones como lo harían las apps. Comprueban las reglas que más importan: que no haya doble canje, que el cupo se respete, que cada error sea el correcto.
- **De concurrencia:** lanzan 20 peticiones **al mismo tiempo** para comprobar que solo una gana. Es la prueba que te deja dormir tranquilo.

Además, cada respuesta se compara contra el contrato (`openapi.yaml`): si la API devuelve algo distinto a lo prometido a las apps, la prueba falla.

Al final de este playbook tendrás una tabla con **cada promesa del *Definition of Done*** y la prueba que la demuestra. Esa tabla es tu evidencia de "está listo".

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Pruebas de integración y *build tags* | https://go.dev/doc/build-cover · `//go:build integration` en https://pkg.go.dev/cmd/go#hdr-Build_constraints |
| `TestMain` (preparar una vez para todas las pruebas) | https://pkg.go.dev/testing#hdr-Main |
| testcontainers-go con Postgres | https://golang.testcontainers.org/modules/postgres/ |
| Detector de carreras (`-race`) | https://go.dev/doc/articles/race_detector |
| `t.Parallel`, `t.Cleanup`, `httptest.NewServer` | https://pkg.go.dev/testing · https://pkg.go.dev/net/http/httptest |
| Validar respuestas contra OpenAPI con kin-openapi | https://pkg.go.dev/github.com/getkin/kin-openapi/openapi3filter |

---

## Requisitos previos

- [ ] 08–11 completos con sus pruebas.
- [ ] Docker corriendo.
- [ ] `oapi-codegen` configurado con `embedded-spec: true` (02).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-testing` | Organización, *build tags*, `TestMain`, pruebas paralelas, cobertura, *goleak*. |
| `golang-database` | Contenedor de Postgres, migraciones por prueba, limpieza entre pruebas. |
| `golang-concurrency` | Pruebas con goroutines y barrera de arranque simultáneo. |
| `golang-stretchr-testify` | (Opcional) `require`/`assert` para pruebas más legibles. |
| `golang-troubleshooting` | Cuando una prueba falle de forma intermitente: buscar la causa, nunca "reintentar hasta que pase". |

---

## Plan (paso a paso)

### 1. Infraestructura — `test/integration/main_test.go`

- `//go:build integration` en todos los archivos de la carpeta.
- `TestMain`: arranca **un** contenedor `postgres:17-alpine` (misma versión que producción), aplica las migraciones con `migrations.FS` (goose como librería), crea el `pgxpool`.
- Helper `newAPI(t) *testAPI`: construye la API completa (`httpapi.New(...)`) con un reloj y un notificador falsos, la monta en `httptest.NewServer`, y devuelve un cliente.
- Helper `reset(t)`: `TRUNCATE code_events, codes, offers, campaigns, api_keys, client_apps RESTART IDENTITY CASCADE` al inicio de cada prueba (las pruebas de integración **no** corren en paralelo entre sí).
- Fixtures: `seedPilot(t)` crea apps, keys (devuelve las keys en claro para la prueba), campaña activa y ofertas con cupo configurable.

### 2. Validación de contrato

Un `http.RoundTripper` envolvente para el cliente de prueba que, en cada respuesta, ejecuta `openapi3filter.ValidateResponse` contra la especificación incrustada. Cualquier desviación hace fallar la prueba con el detalle.

### 3. Concurrencia — `test/integration/concurrency_test.go`

- **Canje:** emitir un código; lanzar 20 goroutines que esperan una señal común (`chan struct{}` cerrado a la vez) y llaman `redeem` con **correos distintos**; contar respuestas → exactamente **1×200** y **19×409 `CODE_ALREADY_REDEEMED`**.
- **Cupo:** oferta con `total_quota = 5`; 20 goroutines emiten para 20 `holderRef` distintos → exactamente **5×201** y **15×409 `OFFER_SOLD_OUT`**; en la BD hay 5 códigos `ISSUED`.
- **Reemisión concurrente:** 10 goroutines emiten para el **mismo** `holderRef` → al final hay **1** código `ISSUED` y 9 `REVOKED` (nunca dos vigentes).
- Ejecutar con `-race` y `-count=10` para descartar resultados intermitentes.

### 4. Mapa del *Definition of Done* → pruebas

Completa esta tabla con el nombre real de cada prueba y cópiala en el PR / en `docs/playbooks/README.md`:

| # | Casilla del DoD (SD §0.2) | Prueba |
|:-:|---|---|
| 1 | `GET /v1/offers`: ECF → ofertas; POS → 403 | `TestOffers_ScopeMatrix` |
| 2 | `POST /v1/codes`: 40 dígitos, Luhn válido; en BD solo hash y last4 | `TestIssue_StoresOnlyHash` |
| 3 | Pedir otra vez → nuevo; anterior `REVOKED`; canjearlo → 410 | `TestIssue_ReissueRevokesPrevious` |
| 4 | Ya canjeado → 409 `HOLDER_LIMIT_REACHED` | `TestIssue_HolderLimit` |
| 5 | `validate` con un dígito cambiado → 422 sin consultar BD | `TestValidate_LuhnRejectsWithoutDB` |
| 6 | `redeem` guarda correo en minúsculas y consentimiento; segundo → 409 | `TestRedeem_HappyPathAndTwice` |
| 7 | 20 goroutines → exactamente un 200 | `TestRedeem_Concurrent20` |
| 8 | Código de otra app destino → 404 | `TestRedeem_OtherTargetApp404` |
| 9 | Mismo correo, misma oferta → 409 `EMAIL_ALREADY_REDEEMED` | `TestRedeem_EmailOncePerOffer` |
| 10 | CSV de canjeados; `fulfill` → `FULFILLED` | `TestAdmin_CSVAndFulfill` |
| 11 | > 10 validaciones fallidas/min → 429 | `TestRateLimit_ValidateFailures` |
| 12 | `/healthz`, `/readyz`; logs JSON con `request_id` y app, sin códigos ni correos | `TestOps_HealthAndLogsRedaction` |

Para la casilla 12, la API de prueba escribe logs a un `bytes.Buffer`; la prueba busca `request_id` y `client_app`, y verifica que **no** aparezcan el código, la key ni el correo completos.

### 5. Comandos

```makefile
test:      ## unitarias
	go test -race ./...
test-int:  ## integración (requiere Docker)
	go test -tags=integration -race -count=1 ./test/integration/...
cover:     ## cobertura combinada
	go test -tags=integration -race -coverpkg=./internal/... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
```

---

## Cómo saber que quedó bien

```bash
make test
make test-int
go test -tags=integration -race -run 'Concurrent|Quota' -count=10 ./test/integration/...
make cover          # objetivo orientativo: ≥ 80 % en internal/service e internal/codes
```

- [ ] Las 12 casillas del DoD tienen prueba y pasan.
- [ ] Las pruebas de concurrencia pasan 10 veces seguidas con `-race`.
- [ ] Ninguna respuesta viola el contrato.
- [ ] `make test` (sin Docker) sigue funcionando y es rápido (< 10 s).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `Cannot connect to the Docker daemon` | Docker apagado | Encender Docker |
| Pruebas que pasan solas pero fallan juntas | Datos de una prueba afectan a otra | `reset(t)` al inicio de cada prueba |
| La prueba concurrente a veces da 2 éxitos | Bug real: lectura antes del `UPDATE` | **No** reintentar; revisar 09 §3 con `golang-troubleshooting` |
| Validación de contrato falla por `null` | Campo no marcado `nullable` en el YAML | Corregir el contrato (y avisar a las apps) o la respuesta |
| En Mac, testcontainers no encuentra Docker (Colima/OrbStack) | Variable `DOCKER_HOST` | Ver la guía de testcontainers para tu runtime |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-testing, golang-database, golang-concurrency).
Implementa el playbook docs/playbooks/12-pruebas-integracion.md: infraestructura con
testcontainers y TestMain, validación de respuestas con kin-openapi, las pruebas de
concurrencia del paso 3 y completa la tabla del paso 4 con los nombres reales.
Si alguna prueba falla, usa golang-troubleshooting para encontrar la causa raíz; no
la marques como "flaky". Al final explícame cómo funciona la barrera de arranque
de la prueba concurrente.
```

---

## Próximos pasos

- ➡️ **13 — Construir** y **15 — Producción**.
- ➡️ **14 — CI** (después del piloto): estas mismas pruebas correrán en cada cambio.
- Antes de producción, pide una revisión: `Usa golang-security en modo auditoría sobre internal/` y, si es posible, la revisión humana que recomienda el SD (§6.3 y §5.1).
