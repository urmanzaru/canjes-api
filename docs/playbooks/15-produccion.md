# 15 · Lanzar a producción — ¿Cómo lanzarlo a producción?

| | |
|---|---|
| **Objetivo** | Publicar `canjes-api` con HTTPS en un dominio de la empresa, con Postgres administrado y respaldos, migraciones aplicadas, catálogo del piloto sembrado, keys entregadas de forma segura y una prueba de punta a punta superada. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 4 (dom 4 oct). **Límite: antes de que salga la versión de Emprende con Flow el lun 5 oct.** |
| **Depende de** | 12, 13 (14 opcional) |
| **Desbloquea** | 16 |
| **Secciones del SD** | §1.2, §7.5, §8, §9, §11 (A1, A3, A4, A5, A7), "Integración en las apps" |
| **Tiempo estimado** | 3–4 h la primera vez + 2 h de estudio |

---

## Notas para ti (sin tecnicismos)

Hasta ahora la API vivía en tu computadora. Producción es **abrir el local al público**: un servidor en la nube que está encendido siempre, con candado (HTTPS), una base de datos cuidada por el proveedor (con copias de seguridad diarias) y una dirección fija como `https://canjes.<dominio-de-la-empresa>`.

El orden importa, como en una mudanza:

1. **Rentar el local** (plataforma de contenedores) y **la bodega** (Postgres administrado).
2. **Guardar las llaves en la caja fuerte** (contraseña de la base como *secreto*, nunca en el código).
3. **Subir la caja de envío** (la imagen Docker del playbook 13).
4. **Acomodar los estantes** (migraciones) y **el inventario inicial** (semilla del piloto).
5. **Abrir la puerta** (desplegar) y **poner el letrero** (dominio con HTTPS).
6. **Entregar los gafetes** (API keys) a cada app por un canal seguro.
7. **Hacer un simulacro** de punta a punta antes de que lleguen las personas.

> **Decisión pendiente A1 (SD §11):** ¿dónde se despliega? Este playbook trae la receta completa para **Google Cloud Run + Cloud SQL** (encaja con "una instancia, contenedor, Postgres administrado") y una alternativa con **un servidor propio (VPS) + Docker Compose + Caddy**. Si la empresa ya usa otra plataforma, los pasos son los mismos con otros comandos: pídele a Claude que los traduzca.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Qué es un servicio administrado y por qué no "instalar Postgres a mano" | Pídele a Claude: "diferencia entre Postgres administrado y autoadministrado" |
| Cloud Run: servicios, revisiones, *jobs* | https://cloud.google.com/run/docs/overview/what-is-cloud-run |
| Cloud SQL para PostgreSQL y conexión desde Cloud Run | https://cloud.google.com/sql/docs/postgres/connect-run |
| Secret Manager | https://cloud.google.com/secret-manager/docs/overview |
| DNS, dominios y certificados HTTPS (la idea) | https://developer.mozilla.org/es/docs/Learn/Common_questions/Web_mechanics/What_is_a_domain_name |
| Leer logs en la nube | https://cloud.google.com/logging/docs/view/logs-explorer-interface |

---

## Requisitos previos

- [ ] 12 completo: todas las pruebas y el DoD en verde.
- [ ] 13 completo: `make docker` produce una imagen `linux/amd64`.
- [ ] Decisión **A1** tomada (plataforma y Postgres) y acceso con permisos para crear recursos y facturar.
- [ ] Acceso al DNS del dominio de la empresa (para `canjes.<dominio>`).
- [ ] Decisiones **A3** (fecha de canje) y **A4** (cupo) reflejadas en `seeds/ecf-piloto-2026-10.sql`; todas las ofertas del piloto definidas.
- [ ] Un gestor de contraseñas para guardar y compartir keys.
- [ ] (Recomendado) Revisión de seguridad hecha (ver paso 0).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-security` | Auditoría previa, secretos, TLS a la base, usuario de BD con privilegios mínimos, cabeceras, IP real detrás del proxy. |
| `golang-observability` | Qué mirar en los logs, alertas mínimas (5xx, `/readyz`, picos de `VALIDATE_FAILED`). |
| `golang-database` | Pool de conexiones acorde a la instancia, migraciones como paso previo, respaldos. |
| `golang-troubleshooting` | Si algo falla en el despliegue. |

---

## Plan (paso a paso)

### 0. Revisión previa (30 min)

```bash
make lint && make test && make test-int && make vuln
```

Y en Claude Code: `Usa golang-security en modo auditoría sobre internal/ y cmd/`. Resuelve todo lo crítico/alto antes de seguir. Si alguien del equipo sabe Go, que revise §6.3 (canje) y §5.1 (auth), como recomienda el SD.

### Opción A — Google Cloud Run + Cloud SQL (recomendada si la empresa usa Google Cloud)

> Los comandos son una guía; confirma *flags* con `gcloud <comando> --help`, porque cambian con el tiempo. Sustituye `PROYECTO`, `REGION` (p. ej. `us-central1`) y contraseñas.

**A1. Proyecto y servicios**

```bash
gcloud config set project PROYECTO
gcloud services enable run.googleapis.com sqladmin.googleapis.com \
  secretmanager.googleapis.com artifactregistry.googleapis.com
```

**A2. Postgres administrado (Cloud SQL)**

```bash
gcloud sql instances create canjes-pg --database-version=POSTGRES_17 \
  --edition=ENTERPRISE --tier=db-f1-micro --region=REGION \
  --backup-start-time=08:00 --enable-point-in-time-recovery
gcloud sql databases create canjes --instance=canjes-pg
gcloud sql users create canjes_app --instance=canjes-pg --password='<contraseña larga aleatoria>'
```

La versión mayor (`17`) debe coincidir con la de `docker-compose.yml` y las pruebas.

**A3. Secreto con la cadena de conexión**

```bash
printf 'postgres://canjes_app:<contraseña>@/canjes?host=/cloudsql/PROYECTO:REGION:canjes-pg' \
  | gcloud secrets create canjes-database-url --data-file=-
```

(Por *socket* de Cloud SQL la conexión ya va cifrada por el conector; si te conectas por IP, usa `sslmode=require` como dice el SD.)

**A4. Registro de imágenes y subida**

```bash
gcloud artifacts repositories create canjes --repository-format=docker --location=REGION
gcloud auth configure-docker REGION-docker.pkg.dev
IMG=REGION-docker.pkg.dev/PROYECTO/canjes/canjes-api:v1.0.0
docker buildx build --platform linux/amd64 --build-arg VERSION=v1.0.0 -t $IMG --push .
```

**A5. Migraciones como *job* (paso previo de cada despliegue)**

```bash
gcloud run jobs create canjes-migrate --image=$IMG --region=REGION \
  --command=/canjesctl --args=migrate,up \
  --set-secrets=DATABASE_URL=canjes-database-url:latest \
  --set-cloudsql-instances=PROYECTO:REGION:canjes-pg
gcloud run jobs execute canjes-migrate --region=REGION --wait
```

En despliegues siguientes: `gcloud run jobs update canjes-migrate --image=$IMG_NUEVA` y `execute`.

**A6. Desplegar el servicio**

```bash
gcloud run deploy canjes-api --image=$IMG --region=REGION --port=8080 \
  --allow-unauthenticated \
  --min-instances=1 --max-instances=1 \
  --set-secrets=DATABASE_URL=canjes-database-url:latest \
  --add-cloudsql-instances=PROYECTO:REGION:canjes-pg \
  --set-env-vars=LOG_LEVEL=info,TRUSTED_PROXY_HOPS=1,RATE_LIMIT_REDEEM_PER_MIN=10,RATE_LIMIT_ISSUE_PER_MIN=30
```

- `--allow-unauthenticated`: la puerta HTTP está abierta porque **la API hace su propia autenticación** con API keys.
- `min=max=1`: una sola instancia siempre encendida (sin "arranque en frío" y el límite de intentos en memoria es coherente, SD §8 borde 9).
- `TRUSTED_PROXY_HOPS=1`: Cloud Run agrega la IP real al final de `X-Forwarded-For`. **Compruébalo** en el paso 9 (el log debe mostrar tu IP pública).
- Añade `NOTIFY_WEBHOOK_URL` como secreto si decidiste A5.

**A7. Dominio con HTTPS**

- Preferido: `canjes.<dominio>` con *domain mapping* de Cloud Run (si tu región lo soporta) o un balanceador HTTPS. El certificado es automático.
- Mínimo viable: la URL `https://canjes-api-xxxx.run.app` ya tiene HTTPS. **Pero** si la app sale con esa URL, cambiarla después exige actualizar la app. Decide antes del lunes.

### Opción B — Servidor propio (VPS) con Docker Compose + Caddy

Para un VPS que la empresa ya tenga (Ubuntu con Docker):

- `docker-compose.prod.yml` con dos servicios: `api` (la imagen, `env_file: .env.prod`, `restart: unless-stopped`) y `caddy` (puertos 80/443, `Caddyfile` con `canjes.<dominio> { reverse_proxy api:8080 }`; Caddy obtiene el certificado HTTPS solo).
- Postgres: **administrado** (Neon, Supabase, Cloud SQL, RDS…) con respaldos y `sslmode=require`. Evita Postgres dentro del mismo VPS salvo que haya respaldos automáticos probados.
- `TRUSTED_PROXY_HOPS=1` (Caddy es el proxy).
- Migraciones: `docker compose run --rm --entrypoint /canjesctl api migrate up` antes de `docker compose up -d`.
- DNS: registro `A` de `canjes.<dominio>` → IP del VPS.

### 8. Sembrar y crear keys (cualquier opción)

Ejecuta `canjesctl` contra producción (como *job* en Cloud Run, o con `docker run … --entrypoint /canjesctl` y la `DATABASE_URL` de producción desde tu máquina por un canal autorizado):

```bash
canjesctl seed seeds/ecf-piloto-2026-10.sql
canjesctl offers list --campaign ecf-piloto-2026-10
canjesctl keys create --app emprende-con-flow --scopes offers:read,codes:issue
canjesctl keys create --app visorus-pos       --scopes codes:redeem
canjesctl keys create --app panel-interno     --scopes admin
```

- Guarda las 3 keys en el gestor de contraseñas **en el momento** (se muestran una vez).
- Comparte la de cada app **solo** con quien la integra, por el gestor de contraseñas.
- La key `admin` es solo tuya.

> El *seed* del piloto debe ir **dentro de la imagen** o ejecutarse desde tu máquina: `canjesctl seed` lee un archivo local. En Cloud Run, lo más simple es ejecutarlo desde tu máquina con el *Cloud SQL Auth Proxy* (`cloud-sql-proxy PROYECTO:REGION:canjes-pg`) y `go run ./cmd/canjesctl seed …`.

### 9. Simulacro de punta a punta (antes del 7 de octubre)

La campaña del piloto empieza a emitir el **7 de octubre**, así que el simulacro usa una campaña de **QA** que no estorba:

1. `canjesctl apps create --slug qa-emisor --name "QA emisor"` + campaña `qa-prod-2026-10` (emisora `qa-emisor`, activa desde hoy, cierra en 2 días) + una oferta con destino `visorus-pos`.
2. `canjesctl keys create --app qa-emisor --scopes offers:read,codes:issue`.
3. Con `curl` contra `https://canjes.<dominio>`:
   - `/healthz` y `/readyz` → 200.
   - `GET /v1/offers` con key QA → oferta; con key POS → 403.
   - `POST /v1/codes` → 201; repetir → `reissued: true`.
   - `validate` con un dígito cambiado → 422; correcto → 200.
   - `redeem` con **tu** correo → 200; segundo → 409.
   - `GET /v1/admin/codes?campaign=qa-prod-2026-10&format=csv` con key admin → tu canje.
   - `fulfill` → 200.
4. **Con las dos apps reales** (versiones de prueba apuntando a producción): emisión en Emprende con Flow con la key QA → canje en VisorUS POS con la key real del POS.
5. En los logs: una línea por petición, con `request_id` y `client_app`, **sin** códigos ni correos completos; la IP es la real.
6. Limpieza: `canjesctl campaigns close --slug qa-prod-2026-10` y `canjesctl keys revoke --prefix <key QA>`.

### 10. Monitoreo mínimo

- **Chequeo de disponibilidad** (Uptime check) cada 5 min sobre `https://canjes.<dominio>/readyz` → alerta por correo si falla.
- **Alerta por logs:** cualquier línea con `status >= 500`.
- **Revisión diaria** de `/v1/admin/stats` (playbook 16).

### 11. Plan de vuelta atrás (*rollback*)

- **Cloud Run:** cada despliegue es una revisión; volver a la anterior:
  `gcloud run services update-traffic canjes-api --to-revisions=<revisión-anterior>=100 --region=REGION`.
- **VPS:** cambiar la etiqueta de imagen en el compose y `docker compose up -d`.
- **Migraciones:** en el piloto solo hay `00001`. Regla para el futuro: migraciones **compatibles hacia atrás** (agregar columnas, no renombrar/borrar en el mismo despliegue).
- **Datos:** restauración desde respaldo / *point-in-time recovery*. **Prueba una restauración una vez** en una instancia temporal; un respaldo que nunca se probó no es un respaldo.

### 12. Comunicar

- A Emprende con Flow: URL base, key (por gestor), recordatorio de "descontar puntos solo tras 201".
- A VisorUS POS: URL base, key, tabla de mensajes de SD §5.2, texto del consentimiento (A7) y `consentVersion = pos-canje-v1`.
- Actualizar `SD_Piloto_Canje_Promociones_Externas.md` (SD, nota inicial).

---

## Cómo saber que quedó bien

- [ ] `https://canjes.<dominio>/readyz` → 200 desde tu teléfono (fuera de la red de la oficina).
- [ ] `canjesctl migrate status` en producción muestra `00001` aplicada.
- [ ] El catálogo del piloto aparece en `canjesctl offers list --campaign ecf-piloto-2026-10` con fechas y cupos correctos.
- [ ] Las 3 keys de producción están en el gestor de contraseñas y no aparecen en ningún chat, correo, archivo del repo ni log.
- [ ] El simulacro del paso 9 completo, incluida la prueba con las dos apps.
- [ ] Respaldos automáticos activos; restauración probada al menos una vez.
- [ ] Alerta de disponibilidad configurada y probada (detén el servicio un momento y espera el correo, o usa la función de prueba del proveedor).
- [ ] Sabes cómo volver a la revisión anterior (lo ensayaste).

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| El servicio no arranca: `DATABASE_URL` vacío | Secreto no montado | `--set-secrets` y permiso *Secret Accessor* para la cuenta de servicio de Cloud Run |
| `/readyz` 503 en Cloud Run | Falta `--add-cloudsql-instances` o el usuario/contraseña | Revisar la cadena de conexión y los logs del servicio |
| `exec format error` | Imagen `arm64` | Reconstruir con `--platform linux/amd64` |
| Todas las peticiones caen en el mismo límite de intentos | IP mal calculada detrás del proxy | Ajustar `TRUSTED_PROXY_HOPS` y verificar en el log |
| `CAMPAIGN_NOT_ACTIVE` al probar el piloto | La campaña del piloto empieza el 7 oct | Correcto; usar la campaña QA del paso 9 |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-security, golang-observability, golang-database).
Acompáñame en el playbook docs/playbooks/15-produccion.md con la opción <A: Cloud Run + Cloud SQL / B: VPS + Caddy>.
Mi proyecto/servidor es <…> y mi dominio <…>. Primero haz la revisión del paso 0.
Luego dame los comandos uno por uno, explicando qué hace cada uno antes de que lo ejecute;
nunca me pidas pegar contraseñas o keys en el chat.
```

---

## Próximos pasos

- ➡️ **16 — Operar el piloto** (7–20 de octubre).
- Crea el *tag* `v1.0.0` del commit desplegado.
- Después del piloto: **14 — CI** y, si el uso crece, revisar SD §10 (JWT, varias instancias, Redis).
