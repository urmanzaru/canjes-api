# 06 · CLI `canjesctl` y semilla del piloto

| | |
|---|---|
| **Objetivo** | Construir `canjesctl` para: aplicar migraciones, dar de alta apps, keys (se muestran una sola vez), campañas y ofertas, y cargar la semilla del piloto. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1–2 (jue 1 / vie 2 oct) |
| **Depende de** | 03 (consultas y migraciones), 04 (`GenerateAPIKey`, `Hash`) |
| **Desbloquea** | 07 (sin keys no se puede probar la autenticación), 15 |
| **Secciones del SD** | D4, D14, RF7, §6.5 (creación de keys), §7.2, §11 (A3, A4) |
| **Tiempo estimado** | 3 h + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

`canjesctl` es **tu panel de control en la terminal**. En lugar de construir una página web de administración (una semana de trabajo), tienes comandos cortos:

```bash
canjesctl apps create --slug visorus-pos --name "VisorUS POS"
canjesctl keys create --app visorus-pos --scopes codes:redeem
```

Lo más delicado son las **API keys**: son como las llaves maestras de cada app. El comando la muestra **una sola vez**; la base de datos solo guarda su huella (hash). Si la pierdes, no se puede recuperar: se crea otra y se revoca la anterior. **Guárdalas en un gestor de contraseñas** (1Password, Bitwarden) y compártelas con los equipos de las apps por ese medio, nunca por chat o correo.

La **semilla** es el "inventario inicial" del piloto: las apps, la campaña de octubre y sus ofertas, listos para cargar con un solo comando en tu máquina o en producción.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Argumentos de línea de comandos y `flag` | https://gobyexample.com/command-line-flags · https://gobyexample.com/command-line-subcommands |
| `os.Exit`, códigos de salida, escribir en `stderr` | https://gobyexample.com/exit |
| `embed` (archivos dentro del binario) | https://pkg.go.dev/embed |
| Fechas en Go (`time.Parse`, RFC 3339, zonas horarias) | https://gobyexample.com/time-formatting-parsing |
| Zona horaria de CDMX (sin horario de verano desde 2022 → UTC−6) | Contexto para entender las fechas de la semilla |

---

## Requisitos previos

- [ ] 03 completo (consultas de `apps`, `keys`, `campaigns`, `offers` generadas; `migrations.FS` existe).
- [ ] 04 completo (`codes.GenerateAPIKey`, `codes.Hash`).
- [ ] Decisiones A3 y A4 del SD confirmadas (fecha límite de canje y cupo por oferta).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-cli` | Estructura de subcomandos, *flags*, códigos de salida, mensajes a `stderr`, ayuda clara. |
| `golang-security` | Mostrar la key una sola vez, nunca registrarla, guardar solo hash. |
| `golang-database` | Migraciones con goose como librería; semilla en una transacción. |
| (`golang-spf13-cobra`) | **Solo si** decides usar cobra. Recomendación: `flag` estándar; son ~10 comandos y no justifican otra dependencia. |

---

## Plan (paso a paso)

### 1. Estructura de comandos

| Comando | Qué hace |
|---|---|
| `canjesctl migrate up \| down \| status` | goose como librería con `goose.SetBaseFS(migrations.FS)`, dialecto `postgres`, conexión con `pgx/v5/stdlib`. |
| `canjesctl apps create --slug --name` / `apps list` | Alta y listado de apps cliente. |
| `canjesctl keys create --app <slug> --scopes a,b` | Genera key, guarda `sha256` + prefijo + scopes, **imprime la key una vez** con aviso. Valida scopes contra la lista permitida: `offers:read`, `codes:issue`, `codes:redeem`, `admin`. |
| `canjesctl keys list [--app]` | Prefijo, app, scopes, fecha, revocada sí/no. **Nunca** la key. |
| `canjesctl keys revoke --prefix ck_live_XXXX` | Marca `revoked_at`. Avisa: "puede tardar hasta 60 s en surtir efecto" (caché del playbook 07). |
| `canjesctl campaigns create --slug --name --issuer <app> --starts --ends --redeem-until` | Fechas en RFC 3339. Queda en `DRAFT`. |
| `canjesctl campaigns activate \| close --slug` | Cambia estado (`DRAFT → ACTIVE → CLOSED`). Al activar, rechaza si la app emisora ya tiene otra campaña `ACTIVE` (ver nota en 08). |
| `canjesctl offers create --campaign --target <app> --slug --title --benefits '<json>' [--description --cost-points --max-per-holder --quota --ios --android --sort]` | Valida que `benefits` sea JSON con `type` y `value`. |
| `canjesctl offers list --campaign` | Para revisar el catálogo. |
| `canjesctl seed <archivo.sql>` | Ejecuta el SQL en una transacción (evita necesitar `psql` en producción). |

Configuración: lee `DATABASE_URL` igual que la API (`internal/config`). Salida: datos a `stdout`, mensajes a `stderr`; código de salida ≠ 0 si falla.

Organización: `cmd/canjesctl/main.go` (enrutado de subcomandos) + un archivo por grupo (`migrate.go`, `apps.go`, `keys.go`, `campaigns.go`, `offers.go`, `seed.go`).

### 2. Semilla del piloto — `seeds/ecf-piloto-2026-10.sql`

Con `INSERT … ON CONFLICT (slug) DO NOTHING` para poder ejecutarla dos veces sin duplicar:

| Qué | Valor |
|---|---|
| Apps | `emprende-con-flow`, `visorus-pos`, `panel-interno` (dueña de la key `admin`) |
| Campaña | `ecf-piloto-2026-10`, emisora `emprende-con-flow`, `ACTIVE` |
| Emisión | `starts_at = 2026-10-07T06:00:00Z` (7 oct 00:00 CDMX) · `ends_at = 2026-10-21T05:59:00Z` (20 oct 23:59 CDMX) |
| Canje (A3) | `redeem_until = 2026-11-01T05:59:59Z` (31 oct 23:59:59 CDMX) |
| Oferta | `visorus-pos-1m-asesoria` · "1 mes gratis de VisorUS POS + 1 h de asesoría" · `benefits = [{"type":"FREE_MONTHS","value":1},{"type":"ADVISORY_HOURS","value":1}]` · `cost_points = 1500` · `max_per_holder = 1` · `total_quota = 20` (A4) · `store_links` reales de iOS/Android |
| Otras ofertas | El SD habla de ~3 ofertas; **agrega aquí las que definas** con el mismo formato |

> Las **keys no van en la semilla** (deben mostrarse una sola vez): se crean con `canjesctl keys create` después de sembrar.

### 3. Semilla de desarrollo — `seeds/dev.sql`

Igual que la del piloto pero con campaña `ecf-prueba` cuyas fechas empiezan **ya** y terminan en un año, para poder probar todo antes del 7 de octubre. **Nunca** se carga en producción (o, si se usa para la prueba de punta a punta del playbook 15, se cierra después).

### 4. Pruebas

- Unitarias: validación de scopes, parseo de fechas y de `benefits`.
- Integración (etiqueta `integration`, se completan en el 12): `keys create` guarda el hash y no la key; `seed` dos veces no duplica.

---

## Cómo saber que quedó bien

```bash
go run ./cmd/canjesctl migrate status
go run ./cmd/canjesctl seed seeds/dev.sql
go run ./cmd/canjesctl apps list
go run ./cmd/canjesctl keys create --app emprende-con-flow --scopes offers:read,codes:issue
go run ./cmd/canjesctl keys create --app visorus-pos --scopes codes:redeem
go run ./cmd/canjesctl keys create --app panel-interno --scopes admin
go run ./cmd/canjesctl keys list
psql "$DATABASE_URL" -c "select key_prefix, scopes from api_keys"
```

- [ ] Cada `keys create` imprime una key `ck_live_…` de 40 caracteres **una vez**.
- [ ] `keys list` y la tabla `api_keys` muestran solo prefijo y hash, nunca la key completa.
- [ ] `keys create --scopes codes:borrar` falla con un mensaje claro y código de salida 1.
- [ ] `seed` dos veces seguidas no duplica nada.
- [ ] `campaigns activate` de una segunda campaña para la misma emisora se rechaza.
- [ ] Guarda las tres keys de desarrollo en tu `.env` local como `DEV_KEY_ECF`, `DEV_KEY_POS`, `DEV_KEY_ADMIN` (para los `curl` de los siguientes playbooks).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| Fechas corridas 6 horas | Se escribió la hora local sin zona | Siempre RFC 3339 con `Z` (UTC) o con `-06:00` |
| `violates check constraint campaigns_check` | `starts_at < ends_at ≤ redeem_until` no se cumple | Revisa el orden de las fechas |
| Perdiste una key | Solo se muestra una vez | `keys create` otra y `keys revoke` la vieja |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-cli, golang-security, golang-database).
Implementa el playbook docs/playbooks/06-cli-canjesctl.md con el paquete flag estándar.
Las keys se imprimen una sola vez y solo se guarda su hash (SD §6.5).
Crea seeds/ecf-piloto-2026-10.sql y seeds/dev.sql con los valores de la tabla del paso 2.
Al terminar, ejecuta las comprobaciones y explícame cómo funciona un subcomando con flag.FlagSet.
```

---

## Próximos pasos

- ➡️ **07 — Autenticación**: con las keys creadas aquí ya se puede probar 401/403.
- 📝 Define las demás ofertas del piloto (título, beneficios, enlaces de tienda) y agrégalas a la semilla.
- En producción (15) el orden será: `migrate up` → `seed seeds/ecf-piloto-2026-10.sql` → `keys create` ×3.
