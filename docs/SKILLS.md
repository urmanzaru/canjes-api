# Guía de skills para `canjes-api`

Análisis de [`samber/cc-skills-golang`](https://github.com/samber/cc-skills-golang) (plugin `cc-skills-golang@samber`, v2.0.1) y de su orquestador `/golang-how-to`, aplicado a esta API. La fuente de verdad sigue siendo [`SYSTEM_DESIGN.md`](./SYSTEM_DESIGN.md).

---

## 1. Qué es una skill (sin tecnicismos)

Una **skill** es un "manual de buenas prácticas" que Claude lee justo cuando lo necesita. En lugar de que Claude escriba Go "como se le ocurra", consulta el manual de bases de datos cuando toca SQL, el de seguridad cuando toca API keys, etc. El repositorio de samber reúne ~45 manuales de Go escritos y revisados por una persona; en sus evaluaciones, Claude comete ~40 % menos errores con ellas.

**`golang-how-to` es el índice.** No enseña nada por sí misma: lee lo que estás pidiendo y abre los manuales adecuados (normalmente varios a la vez). Por eso el `CLAUDE.md` de este repo le dice a Claude que **siempre empiece por ella**.

---

## 2. Instalación

Ya está preconfigurado en `.claude/settings.json`: al abrir este repo con Claude Code te propondrá instalar el plugin. Si no aparece, instálalo a mano:

```bash
# dentro de Claude Code
/plugin marketplace add samber/cc
/plugin install cc-skills-golang@samber
```

Alternativa universal (cualquier agente compatible con *Agent Skills*):

```bash
npx skills add https://github.com/samber/cc-skills-golang --all
```

Comprobación: escribe `/golang-how-to` en Claude Code; si autocompleta, está instalada.

**Dependencia recomendada:** `gopls` (el "cerebro" que entiende tu código Go para navegar y renombrar):

```bash
go install golang.org/x/tools/gopls@latest
```

### Configuración aplicada en este repo

Se ejecutó el equivalente a `/golang-how-to configure`:

- **Directiva siempre activa** en `CLAUDE.md`: cargar `golang-how-to` antes de cualquier tarea de Go.
- **Bloque `## Required Go skills`** con 7 skills que aplican a *todo* el proyecto (ver §4). Se eligió un conjunto reducido frente a las 11 ⭐️ recomendadas para no gastar contexto en skills que no aportan aquí (p. ej. `golang-modernize` en un proyecto nuevo, `golang-data-structures` en una API CRUD).

---

## 3. Cómo enruta `golang-how-to` en este proyecto

Para cada tipo de tarea carga una skill **principal** y varias **secundarias**. Aplicado a `canjes-api`:

| Tarea en canjes-api | Principal | También carga | Playbook |
|---|---|---|---|
| Crear la estructura del repo, `go.mod`, Makefile | `golang-project-layout` | `golang-design-patterns`, `golang-dependency-injection`, `golang-lint` | 01 |
| Elegir/actualizar dependencias (`pgx`, `sqlc`…) | `golang-dependency-management` | `golang-pkg-go-dev` | 00, 01 |
| Migraciones y consultas SQL (`goose`, `sqlc`, `pgx`) | `golang-database` | `golang-error-handling`, `golang-security` | 03 |
| Generador de códigos, Luhn, hash | `golang-security` (crypto/rand, SHA-256) | `golang-testing`, `golang-safety` | 04 |
| Config por variables de entorno, `main.go`, apagado ordenado | `golang-design-patterns` | `golang-context`, `golang-dependency-injection` | 05 |
| Logs con `slog`, `request_id`, `/healthz` | `golang-observability` | `golang-error-handling` | 05 |
| CLI `canjesctl` | `golang-cli` | (`golang-spf13-cobra` solo si se decide usar cobra) | 06 |
| Middleware de API key y scopes | `golang-security` | `golang-design-patterns`, `golang-context` | 07 |
| Servicio de emisión (transacción + `FOR UPDATE`) | `golang-database` | `golang-error-handling`, `golang-structs-interfaces` | 08 |
| Canje atómico y mapeo de errores | `golang-database` | `golang-error-handling`, `golang-concurrency` | 09 |
| Límite de intentos y webhook en segundo plano | `golang-concurrency` | `golang-context`, `golang-security` | 10 |
| Endpoints admin, CSV | `golang-design-patterns` | `golang-security`, `golang-error-handling` | 11 |
| Pruebas unitarias, de integración y concurrencia | `golang-testing` | `golang-database` (testcontainers), `golang-stretchr-testify` | 04–12 |
| Dockerfile, build reproducible | `golang-project-layout` | `golang-security` | 13 |
| GitHub Actions (lint, test, govulncheck) | `golang-continuous-integration` | `golang-lint`, `golang-security` | 14 |
| Algo falla y no sé por qué | `golang-troubleshooting` | `golang-safety` | cualquiera |
| Revisión de seguridad antes de producción | `golang-security` | `golang-safety`, `golang-lint` | 15 |

---

## 4. Skills del proyecto: cuáles sí, cuáles no

### Siempre activas (en `CLAUDE.md`)

| Skill | Por qué aplica a *toda* la API |
|---|---|
| `golang-error-handling` | Cada endpoint traduce errores de dominio a `{error, message}` (D13). Hay que envolver con `%w` y distinguir con `errors.Is/As`. |
| `golang-security` | API keys, hashes, `crypto/rand`, PII (correos) en logs, límite de intentos. Es el corazón del diseño (D4, D6, §9). |
| `golang-database` | Todas las reglas críticas viven en SQL: transacciones, `FOR UPDATE`, índices únicos, códigos de error `23505`. |
| `golang-testing` | El *Definition of Done* son pruebas: unitarias, integración con Postgres real y concurrencia. |
| `golang-safety` | Evita *panics* por `nil` y errores silenciosos, que en una API de canjes significan un 500 en plena demo. |
| `golang-naming` · `golang-code-style` | Consistencia; tú vas a leer este código para aprender, así que conviene que siga el estilo idiomático. |

### Se cargan solas cuando toca (vía `golang-how-to`)

`golang-project-layout`, `golang-design-patterns`, `golang-dependency-injection`, `golang-structs-interfaces`, `golang-context`, `golang-concurrency`, `golang-observability`, `golang-cli`, `golang-lint`, `golang-continuous-integration`, `golang-dependency-management`, `golang-troubleshooting`, `golang-gopls`, `golang-pkg-go-dev`, `golang-documentation`, `golang-stretchr-testify`.

### No aplican (no las uses aquí)

| Skill | Por qué no |
|---|---|
| `golang-swagger` | Usa `swaggo/swag`, que **genera** el OpenAPI desde comentarios en el código. Aquí es al revés: el contrato `openapi.yaml` manda y `oapi-codegen` genera el código (D2). |
| `golang-grpc`, `golang-graphql` | La API es REST/JSON. |
| `golang-google-wire`, `golang-uber-dig`, `golang-uber-fx`, `golang-samber-do` | Contenedores de inyección de dependencias. Con 4–5 componentes, la inyección manual en `main.go` es más clara (lo confirma la propia `golang-dependency-injection`: "cuándo vale la pena la complejidad"). |
| `golang-spf13-viper` | La configuración son 6 variables de entorno (§7.3); `os.Getenv` basta. |
| `golang-samber-lo/mo/ro/hot/oops/slog` | Librerías de samber que no están en el stack. `slog` estándar es suficiente. |
| `golang-benchmark`, `golang-performance` | El piloto son ~30 códigos; la meta de < 100 ms P95 se cumple con índices. Útiles solo si algún día hay un problema de rendimiento medido. |

---

## 5. Huecos y conflictos con el system design

Las skills no cubren tres herramientas del stack y en dos puntos recomiendan algo distinto. **Regla: se aplica el principio de la skill con la herramienta del system design.**

| Tema | Lo que dice la skill | Lo que dice el SD | Qué hacer |
|---|---|---|---|
| Migraciones | `golang-database` recomienda `golang-migrate`, Flyway o Atlas | `goose` (§7.2, §7.5) | Usar **goose**. El principio (migraciones versionadas, escritas y revisadas por personas, aplicadas antes del despliegue) se cumple igual. |
| Diseño de esquema | `golang-database`: "la IA no debe crear esquemas" | El esquema ya está escrito y razonado en §4 | Copiar el SQL de §4 **tal cual**. Cualquier cambio de esquema lo decides tú y se documenta en el SD. |
| Generación de código SQL | `golang-database` menciona `sqlc` solo de pasada | `sqlc` + `pgx/v5` (D3) | Seguir la documentación oficial de sqlc (motor `postgresql`, `sql_package: pgx/v5`). Las reglas de la skill (contexto, `ErrNoRows`, parámetros) siguen aplicando. |
| Contrato OpenAPI | `golang-swagger` / `golang-documentation` proponen `swaggo` | `oapi-codegen` contrato-primero (D2) | **Ninguna skill cubre oapi-codegen.** Seguir su README (generador `std-http-server`, modo *strict*). |
| Pruebas unitarias | `golang-testing`: unitarias con *mocks*, sin BD | Unitarias para `codes` y mapeo de errores; el servicio se prueba contra **Postgres real** (§7.4) | Compatible: el paquete `codes` no toca BD; las reglas de §6.2/§6.3 son SQL y solo tienen sentido probadas contra Postgres (etiqueta `integration`). |

---

## 6. Cómo pedirle trabajo a Claude en este repo

Plantilla para cada playbook:

```text
Usa /golang-how-to. Implementa el playbook docs/playbooks/NN-<nombre>.md.
La fuente de verdad es docs/SYSTEM_DESIGN.md (secciones §X, §Y).
Trabaja en pasos pequeños, corre las pruebas al final de cada paso y
al terminar explícame en lenguaje sencillo qué hiciste y qué concepto
de Go debería estudiar.
```

Otros usos útiles:

- **"No entiendo este archivo"** → `Explícame internal/service/issue.go línea por línea, como si no supiera Go.`
- **"Algo falla"** → `Usa golang-troubleshooting. La prueba X falla con este error: …`
- **"¿Está bien?"** → `/code-review` sobre tus cambios, o `Usa golang-security y revisa el middleware de auth.`
- **"¿Qué skill aplica?"** → `/golang-how-to` y describe la duda; si dos skills parecen chocar, pide el modo *disambiguate*.
