# 00 · Preparar tu máquina — ¿Cómo instalarlo en local?

| | |
|---|---|
| **Objetivo** | Tener instaladas y funcionando todas las herramientas para desarrollar, probar y construir `canjes-api` en tu computadora. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1 (jue 1 oct), primera hora |
| **Depende de** | — |
| **Desbloquea** | 01 |
| **Secciones del SD** | Encabezado (stack), §7.5 |
| **Tiempo estimado** | 1–2 h de instalación + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Antes de cocinar hay que tener la cocina lista. Este playbook no escribe nada de la API: solo instala las "herramientas de cocina":

- **Go** es el idioma en el que se escribe la API y el "traductor" que la convierte en un programa ejecutable.
- **Docker** es una caja que permite correr programas (como la base de datos PostgreSQL) sin instalarlos "de verdad" en tu computadora. Si algo se descompone, borras la caja y creas otra.
- **Git** guarda el historial de cambios del código y lo sube a GitHub.
- **sqlc**, **oapi-codegen** y **goose** son "máquinas que escriben código" a partir de archivos que tú controlas (el SQL y el contrato). Te ahorran escribir a mano cientos de líneas repetitivas.
- **golangci-lint** es un revisor automático que señala errores comunes.
- **Claude Code + skills de Go** es tu copiloto, con manuales de buenas prácticas de Go cargados.

Al terminar, tu máquina podrá correr la API y su base de datos sin depender de nadie.

---

## Qué estudiar para entenderlo

| Tema | Por qué | Recurso |
|---|---|---|
| Terminal básica (`cd`, `ls`, `mkdir`, variables de entorno) | Todo se ejecuta desde la terminal | Busca "command line crash course" o pídele a Claude un resumen |
| Git básico (`clone`, `status`, `add`, `commit`, `push`, ramas) | Guardar y subir tu avance | https://git-scm.com/book/es/v2 (capítulos 1–3) |
| Qué es Docker: imagen vs contenedor | Postgres local y la imagen final de la API | https://docs.docker.com/get-started/ |
| Qué es Go y cómo se instala | Entender `go version`, `GOPATH`, `go install` | https://go.dev/doc/install |

---

## Requisitos previos

- Computadora con **macOS**, **Linux** o **Windows 10/11 con WSL2** (en Windows, trabaja *dentro* de WSL2/Ubuntu; las instrucciones de Linux aplican ahí).
- 10 GB libres en disco.
- Cuenta de GitHub con acceso a `urmanzaru/canjes-api`.
- Permisos de administrador para instalar programas.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-how-to` | Punto de entrada; confirmar que el plugin quedó instalado. |
| `golang-stay-updated` | Saber dónde consultar la versión estable de Go y novedades. |
| `golang-dependency-management` | Entender cómo se instalan herramientas con `go install` y `go get -tool`. |

---

## Plan (paso a paso)

### 1. Gestor de paquetes

- **macOS:** instala Homebrew → https://brew.sh
- **Linux / WSL2 (Ubuntu):** `sudo apt update && sudo apt install -y build-essential curl git make`

### 2. Go (estable más reciente)

```bash
# macOS
brew install go
# Linux / WSL2: descarga el .tar.gz de https://go.dev/dl y sigue https://go.dev/doc/install
```

Agrega las herramientas de Go a tu `PATH` (en `~/.zshrc` o `~/.bashrc`):

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### 3. Git

```bash
git --version            # si no existe: brew install git / sudo apt install git
git config --global user.name  "Uriel Manzano"
git config --global user.email "<tu correo de GitHub>"
```

### 4. Docker

- **macOS / Windows:** Docker Desktop (https://www.docker.com/products/docker-desktop/) u OrbStack (macOS, más ligero).
- **Linux:** Docker Engine + plugin compose (https://docs.docker.com/engine/install/).

### 5. Herramientas de generación y calidad

```bash
# sqlc (genera Go a partir de SQL)
brew install sqlc                      # Linux: https://docs.sqlc.dev/en/latest/overview/install.html

# golangci-lint (revisor automático)
brew install golangci-lint             # Linux: https://golangci-lint.run/welcome/install/

# gopls (entiende tu código Go; lo usan el editor y Claude)
go install golang.org/x/tools/gopls@latest

# govulncheck (busca vulnerabilidades conocidas en dependencias)
go install golang.org/x/vuln/cmd/govulncheck@latest
```

> `oapi-codegen` y `goose` **no** se instalan globalmente: en el playbook 01 se agregan al proyecto con `go get -tool`, así todos usan la misma versión con `go tool oapi-codegen` y `go tool goose`.

### 6. Cliente de base de datos (opcional, muy recomendado para aprender)

- Terminal: `brew install libpq && brew link --force libpq` (te da `psql`).
- Visual: TablePlus, DBeaver o la extensión de PostgreSQL de VS Code.

### 7. Editor

- **VS Code** + extensión oficial **Go** (de Google). Al abrir un `.go`, acepta instalar las herramientas que propone.
- Activa "format on save" (usa `gofmt`/`goimports`).

### 8. Claude Code y las skills de Go

```bash
# dentro de Claude Code, en la carpeta del repo
/plugin marketplace add samber/cc
/plugin install cc-skills-golang@samber
```

El repo ya trae `.claude/settings.json` que propone instalarlo automáticamente. Más detalle en [`docs/SKILLS.md`](../SKILLS.md).

### 9. Clonar el repositorio

```bash
mkdir -p ~/code && cd ~/code
git clone git@github.com:urmanzaru/canjes-api.git   # o https://github.com/urmanzaru/canjes-api.git
cd canjes-api
```

---

## Cómo saber que quedó bien

Ejecuta y compara:

```bash
go version                 # go version go1.2x.x …   (la estable más reciente)
git --version              # git version 2.x
docker run --rm hello-world   # "Hello from Docker!"
docker compose version     # Docker Compose version v2.x
sqlc version               # v1.x
golangci-lint --version    # golangci-lint has version 2.x
gopls version              # golang.org/x/tools/gopls v0.x
govulncheck -version       # Go: go1.2x …
```

- [ ] Todos los comandos responden sin "command not found".
- [ ] `docker run hello-world` imprime el saludo.
- [ ] En Claude Code, escribir `/golang-how-to` autocompleta.
- [ ] El repo está clonado y `git status` responde.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| `gopls: command not found` después de instalarlo | `$(go env GOPATH)/bin` no está en el `PATH` | Agrega la línea del paso 2 a tu `~/.zshrc`/`~/.bashrc` y abre otra terminal |
| `Cannot connect to the Docker daemon` | Docker Desktop no está abierto | Ábrelo y espera a que diga "running" |
| En Windows todo es lento o raro con rutas | Trabajas fuera de WSL2 | Clona el repo dentro de WSL2 (`/home/<tú>/code`), no en `C:\` |
| Mac con chip M1/M2/M3 | Imágenes `arm64` vs servidores `amd64` | No afecta aquí; se resuelve al construir en el playbook 13 |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to. Ayúdame con el playbook docs/playbooks/00-preparar-entorno.md.
Mi sistema es <macOS / Ubuntu / Windows con WSL2>. Revisa qué herramientas me faltan
ejecutando los comandos de "Cómo saber que quedó bien" y guíame a instalarlas una por una.
```

---

## Próximos pasos

- ➡️ **Playbook 01 — Esqueleto del proyecto.**
- Haz el **Tour of Go** (sección *Basics*) mientras se descargan las instalaciones.
