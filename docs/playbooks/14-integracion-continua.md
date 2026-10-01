# 14 · Integración continua (GitHub Actions)

| | |
|---|---|
| **Objetivo** | Que cada *push* y cada PR ejecuten automáticamente: código generado al día, lint, pruebas unitarias e integración, escaneo de vulnerabilidades y construcción de la imagen. Opcional: publicar la imagen al crear un *tag*. |
| **Prioridad** | 🟢 Puede esperar a después del piloto (muy recomendado antes de sumar más personas al proyecto) |
| **Día sugerido** | Semana del 21 de octubre |
| **Depende de** | 12, 13 |
| **Desbloquea** | Despliegues automáticos (extensión del 15) |
| **Secciones del SD** | §7.4, §7.5 |
| **Tiempo estimado** | 2 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Hoy, para saber si algo se rompió, tú corres `make test`. Con CI, **GitHub lo hace por ti cada vez que subes un cambio**, en una computadora limpia, y te pone una ✅ o una ❌ junto al commit. Es un compañero incansable que revisa todo, siempre igual, y que no se salta pasos con prisa.

Revisa cinco cosas:

1. **Que el código generado esté al día** (que nadie olvidó regenerar tras cambiar el contrato o el SQL).
2. **Estilo y errores comunes** (lint).
3. **Pruebas** unitarias y de integración (con Postgres real).
4. **Vulnerabilidades conocidas** en las librerías (`govulncheck`).
5. **Que la imagen Docker se construya.**

Además, **Dependabot** abrirá PRs automáticos cuando una librería tenga una actualización de seguridad.

Se dejó para después del piloto porque no cambia lo que la API hace; cambia **qué tan seguro es cambiarla**. En cuanto el piloto arranque y empieces a hacer ajustes, vale la pena tenerlo.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| GitHub Actions: workflow, job, step | https://docs.github.com/es/actions/learn-github-actions/understanding-github-actions |
| Secretos en GitHub Actions | https://docs.github.com/es/actions/security-guides/using-secrets-in-github-actions |
| Dependabot | https://docs.github.com/es/code-security/dependabot |
| Protección de ramas | https://docs.github.com/es/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches |
| `govulncheck` | https://go.dev/doc/security/vuln/ |

---

## Requisitos previos

- [ ] 12 y 13 completos (`make test`, `make test-int`, `make lint`, `make docker` funcionan en local).
- [ ] Permisos de administración en el repo de GitHub.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-continuous-integration` | Plantillas de `test.yml`, `lint.yml`, `security.yml`, `integration.yml`, `docker.yml`, `dependabot.yml` (en `assets/` de la skill). Adaptarlas, no copiarlas a ciegas. |
| `golang-lint` | Mismo `.golangci.yml` en local y en CI. |
| `golang-security` | `govulncheck`, permisos mínimos del `GITHUB_TOKEN`, acciones fijadas por versión. |
| `golang-dependency-management` | Caché de módulos, `go mod tidy` verificado. |

---

## Plan (paso a paso)

### 1. `.github/workflows/ci.yml` (en cada push y PR a `main`)

| Job | Pasos clave |
|---|---|
| `generate` | `make tools` → `make generate` → `go mod tidy` → `git diff --exit-code` (falla si algo generado no estaba versionado) |
| `lint` | `golangci/golangci-lint-action` con la versión fijada |
| `test` | `actions/setup-go` con `go-version-file: go.mod` → `make test` |
| `integration` | `make test-int` (los *runners* `ubuntu-latest` traen Docker, testcontainers funciona sin configuración) |
| `vuln` | `govulncheck ./...` |
| `docker` | `docker/build-push-action` **sin** publicar (`push: false`), solo comprobar que construye |

Buenas prácticas: `permissions: contents: read` a nivel de workflow; `concurrency` para cancelar ejecuciones viejas del mismo PR; fijar versiones de acciones.

### 2. `.github/dependabot.yml`

Actualizaciones semanales de: `gomod`, `github-actions`, `docker` (imagen base del Dockerfile).

### 3. (Opcional) `.github/workflows/release.yml` — publicar imagen en *tags* `v*`

- Construir `linux/amd64` y publicar en el registro elegido en el 15 (GHCR, Artifact Registry…).
- Autenticación al registro con secretos del repositorio (o *Workload Identity Federation* en Google Cloud, sin llaves).
- Etiquetas: `vX.Y.Z` y `sha-<commit>`.
- El **despliegue** puede seguir siendo manual (recomendado mientras solo tú operas).

### 4. Protección de la rama `main`

Requerir que `generate`, `lint`, `test`, `integration` y `vuln` pasen antes de unir un PR.

### 5. Insignia en el README

`![CI](https://github.com/urmanzaru/canjes-api/actions/workflows/ci.yml/badge.svg)`

---

## Cómo saber que quedó bien

- [ ] Un PR de prueba muestra los 6 *jobs* en verde.
- [ ] Un PR que rompe una prueba a propósito queda en rojo y **no** se puede unir.
- [ ] Un PR que cambia `openapi.yaml` sin regenerar falla en `generate`.
- [ ] Dependabot abre su primer PR (puede tardar unos días).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `generate` falla siempre | Versiones distintas de sqlc/oapi-codegen en local y CI | Fijar versiones (oapi-codegen por `go tool`; sqlc con versión explícita en `make tools`) |
| `lint` da hallazgos que en local no | Versión distinta de golangci-lint | Fijar la misma versión en ambos lados |
| `integration` lento | Descarga de la imagen de Postgres en cada ejecución | Aceptable (~30 s); opcional: caché de imágenes |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-continuous-integration, golang-lint, golang-security).
Implementa el playbook docs/playbooks/14-integracion-continua.md adaptando las plantillas
de la skill a este repo (Makefile existente, testcontainers, código generado versionado).
Permisos mínimos y acciones con versión fijada. Explícame qué hace cada job.
```

---

## Próximos pasos

- Automatizar el despliegue desde `release.yml` cuando haya más de una persona operando.
- Agregar la revisión de PR asistida por IA (plantilla `claude-code-review.yml` de la skill), si te resulta útil.
