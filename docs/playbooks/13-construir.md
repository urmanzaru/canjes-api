# 13 · Construir: binarios e imagen Docker — ¿Cómo construir?

| | |
|---|---|
| **Objetivo** | Compilar los binarios `canjes-api` y `canjesctl` con versión incrustada, y empaquetarlos en una imagen Docker *multi-stage* sobre `distroless/static` (~20 MB) lista para cualquier plataforma de contenedores. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 4 (dom 4 oct) — se puede ensayar desde el 05 |
| **Depende de** | 05 (para tener algo que construir); recomendado tras 12 |
| **Desbloquea** | 14, 15 |
| **Secciones del SD** | D1, §7.2 (Dockerfile, Makefile), §7.5 |
| **Tiempo estimado** | 1–2 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Hasta ahora corrías la API con `make run`, que **traduce y ejecuta** el código cada vez. "Construir" es **traducir una sola vez** y obtener un archivo ejecutable (binario) que se puede llevar a cualquier servidor sin instalar Go.

Luego ese binario se mete en una **caja de envío** (imagen Docker). La caja lleva solo lo indispensable: el programa y los certificados de seguridad. Nada de sistema operativo completo, nada de herramientas extra. Por eso pesa ~20 MB y tiene muy poca superficie de ataque.

La receta tiene dos etapas (*multi-stage*):

1. **Taller:** una caja grande con Go instalado donde se compila.
2. **Caja de envío:** se copia solo el resultado a una caja mínima. El taller se tira.

La misma caja sirve en tu máquina, en Cloud Run, en Fly.io o en un servidor propio: "si funciona aquí, funciona allá".

> **Ojo si tu Mac tiene chip Apple (M1/M2/M3/M4):** tu Mac es `arm64` y la mayoría de servidores son `amd64`. Hay que construir la imagen **para la arquitectura del servidor** (`--platform linux/amd64`), o la nube dirá "exec format error".

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| `go build`, `GOOS`/`GOARCH`, `CGO_ENABLED` | https://go.dev/doc/install/source#environment · https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies |
| Incrustar la versión con `-ldflags -X` | https://pkg.go.dev/cmd/link |
| Dockerfile y *multi-stage builds* | https://docs.docker.com/build/building/multi-stage/ |
| Imágenes *distroless* | https://github.com/GoogleContainerTools/distroless |
| `docker buildx` y plataformas | https://docs.docker.com/build/building/multi-platform/ |

---

## Requisitos previos

- [ ] `make test` y `make test-int` pasan.
- [ ] Docker corriendo.
- [ ] El repo tiene al menos un *tag* o commit (la versión sale de `git describe`).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-project-layout` | Dónde va el `Dockerfile`, `.dockerignore`, objetivos del Makefile. |
| `golang-security` | Imagen mínima, usuario no-root, sin secretos en la imagen, `-trimpath`. |
| `golang-documentation` | Sección "Construir" del README (la skill trae un ejemplo con distroless). |
| `golang-dependency-management` | `go mod download` en capa separada para builds rápidos y reproducibles. |

---

## Plan (paso a paso)

### 1. Binarios locales — `make build`

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/canjes-api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/canjesctl ./cmd/canjesctl
```

- `CGO_ENABLED=0`: binario estático, no depende de librerías del sistema.
- `-trimpath`: no incrusta rutas de tu computadora.
- `-s -w`: quita símbolos de depuración (binario más chico).
- `main.version`: `canjes-api -version` y el log de arranque muestran la versión.

Compilar para Linux desde tu Mac: `GOOS=linux GOARCH=amd64 make build`.

### 2. `.dockerignore`

```
.git
bin/
.env
coverage.*
docs/
test/
*.md
```

### 3. `Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1
ARG GO_VERSION=1.27          # misma versión mayor/menor que go.mod
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/canjes-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/canjesctl ./cmd/canjesctl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/canjes-api /out/canjesctl /
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/canjes-api"]
```

- Ambos binarios en la misma imagen: el servidor es el `ENTRYPOINT`; `canjesctl` se usa cambiando el *entrypoint* (migraciones en el despliegue).
- Las migraciones van **dentro** de `canjesctl` (`embed`), no hace falta copiar archivos.
- `nonroot`: el proceso no corre como administrador.
- No hay `HEALTHCHECK` (distroless no trae `curl`); la plataforma usa `/healthz` y `/readyz`.
- Ajusta `GO_VERSION` a la versión de tu `go.mod` (con `go 1.27` usa `1.27`, etc.).

### 4. Makefile

```makefile
IMAGE ?= canjes-api
docker:
	docker buildx build --platform linux/amd64 --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) --load .
```

### 5. Probar la imagen en local contra el Postgres de Docker Compose

```bash
make docker
docker images canjes-api                      # tamaño ~20 MB
docker run --rm --network host --env-file .env canjes-api:<versión>          # Linux
docker run --rm -p 8080:8080 --env-file .env \
  -e DATABASE_URL="postgres://canjes:canjes@host.docker.internal:5432/canjes?sslmode=disable" \
  canjes-api:<versión>                                                       # macOS / Windows
curl localhost:8080/readyz

# canjesctl desde la misma imagen
docker run --rm --env-file .env -e DATABASE_URL=… --entrypoint /canjesctl canjes-api:<versión> migrate status
```

---

## Cómo saber que quedó bien

- [ ] `make build` produce `bin/canjes-api` y `bin/canjesctl`; `./bin/canjes-api -version` muestra la versión de git.
- [ ] `make docker` termina; la imagen pesa < 30 MB.
- [ ] `docker run …` arranca, `/readyz` responde 200 contra el Postgres local.
- [ ] `docker run --entrypoint /canjesctl … migrate status` funciona.
- [ ] `docker inspect canjes-api:<v> --format '{{.Architecture}}'` → `amd64` (o la de tu servidor).
- [ ] `docker run --rm --entrypoint id canjes-api:<v>` **falla** (no hay shell: es lo esperado en distroless).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `exec format error` en la nube | Imagen `arm64` en servidor `amd64` | `--platform linux/amd64` |
| `connection refused` a Postgres desde el contenedor | `localhost` dentro del contenedor es el contenedor | `host.docker.internal` (Mac/Win) o `--network host` (Linux) |
| `x509: certificate signed by unknown authority` hacia Postgres en la nube | Imagen sin certificados | `distroless/static` ya los trae; no usar `scratch` sin copiarlos |
| Build muy lento cada vez | Se copia todo antes de `go mod download` | Respetar el orden del Dockerfile (dependencias primero) |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-project-layout, golang-security, golang-documentation).
Implementa el playbook docs/playbooks/13-construir.md: make build con versión de git,
Dockerfile multi-stage a distroless nonroot con ambos binarios, .dockerignore y make docker
para linux/amd64. Pruébalo contra el Postgres de docker compose y documenta en el README
la sección "Construir". Explícame cada línea del Dockerfile.
```

---

## Próximos pasos

- ➡️ **15 — Producción**: subir la imagen a un registro y desplegarla.
- ➡️ **14 — CI**: que GitHub construya la imagen automáticamente.
- Crea el primer *tag* cuando despliegues al piloto: `git tag v1.0.0 && git push --tags`.
