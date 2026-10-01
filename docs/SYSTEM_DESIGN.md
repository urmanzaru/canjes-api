# Diseño de Sistema — `canjes-api`: API de emisión y canje de códigos de promoción (Go + PostgreSQL)

**Autor:** Uriel M. · **Fecha:** 1 de octubre de 2026 · **Versión:** 1.0 · **Estado:** listo para implementar
**Quién implementa:** Uriel Manzano (desarrollo, revisión y mantenimiento). **Recomendado:** una revisión de alguien del equipo que ya trabaja en Go, enfocada en §6.3 (canje atómico) y §5.1 (autenticación).
**Primer uso:** piloto de Emprende con Flow (**7 – 20 de octubre de 2026**, 10 personas). Las personas canjean puntos por promociones de las apps de la empresa (p. ej. *1 mes gratis de VisorUS POS + 1 hora de asesoría*).
**Stack:** Go (versión estable más reciente) · PostgreSQL · `net/http` estándar · `pgx/v5` · `sqlc` · `goose` · `oapi-codegen` · `slog` · Docker.

> **Objetivo:** un servicio pequeño y confiable que **emite** códigos de un solo uso desde una app (Emprende con Flow) y los **canjea** en otra (VisorUS POS), guarda **con qué correo** se canjeó cada uno y te deja **ver todo lo canjeado**. Nace como API de emergencia para un piloto, pero el modelo (apps cliente, campañas, ofertas, códigos y bitácora) está pensado para sumar **más apps, más campañas y más tipos de beneficio sin migraciones de fondo**.

> **Nota sobre Emprende con Flow.** Esta API **no toca el backend de Emprende con Flow** ni su contrato. Para Emprende con Flow es un servicio externo de la empresa. El SD del piloto (`SD_Piloto_Canje_Promociones_Externas.md`) se actualizará cuando esta API esté lista.

---

## 0. Alcance y decisiones que lo enmarcan

**Lo que resuelve la v1:**

| # | Necesidad | Cómo |
|---|-----------|------|
| **N1** | Al abrir la pantalla de canjes, la app obtiene las promociones disponibles | `GET /v1/offers` |
| **N2** | Al canjear, la app recibe un **código de 40 dígitos** que **no se puede volver a asignar**, y queda registrado que se emitió | `POST /v1/codes` |
| **N3** | En la otra app (VisorUS POS), **"Tengo un código de canje"**: valida el código, la persona se registra y se marca **canjeado con su correo** | `POST /v1/codes/validate` → registro → `POST /v1/codes/redeem` |
| **N4** | Tú puedes saber **qué se emitió, qué se canjeó, con qué correo y cuándo**, y marcar cuándo entregaste la promoción | `GET /v1/admin/codes` · `POST /v1/admin/codes/{id}/fulfill` |

**Decisiones:**

| # | Decisión | Por qué |
|---|----------|---------|
| **D1** | **Go + PostgreSQL**, un solo binario en un contenedor. | El equipo ya usa Go; arranca en milisegundos, consume poca memoria y no depende de un framework pesado. Postgres da transacciones e índices únicos, que son lo que garantiza que un código no se use dos veces. |
| **D2** | **Contrato primero:** `api/openapi.yaml` es la fuente de verdad y `oapi-codegen` genera los tipos y la interfaz del servidor. | Es la misma disciplina que Emprende con Flow (un OpenAPI que manda). Cambias el contrato, regeneras y el compilador te dice qué falta implementar. |
| **D3** | **SQL escrito a mano, código Go generado con `sqlc`.** Sin ORM. | Las dos reglas críticas (emisión con cupo y canje atómico) son **una sentencia SQL cada una**. Verlas tal cual es más seguro que esconderlas detrás de un ORM. |
| **D4** | **Cada app cliente tiene su propia API key con permisos (scopes).** Emprende con Flow solo puede **leer ofertas y emitir**; VisorUS POS solo puede **validar y canjear**; el panel/CLI interno tiene **admin**. | Si una key se filtra, el daño queda acotado a lo que esa app puede hacer, y se revoca sin afectar a las demás. |
| **D5** | **Código de 40 dígitos numéricos**: 39 aleatorios (`crypto/rand`) + **1 dígito verificador Luhn**. Se muestra en grupos de 4 y se acepta con espacios o guiones. | Es tu requisito. El dígito verificador **detecta errores de tecleo sin consultar la base** (un dígito mal escrito o dos invertidos). 39 dígitos aleatorios son ~129 bits: imposible de adivinar. El formato es **configurable por campaña** (§6.1) por si después quieres uno más corto. |
| **D6** | **En la base se guarda solo el hash SHA-256 del código**, más los últimos 4 dígitos para soporte. **El código completo se muestra una sola vez**, en la respuesta de emisión. | Si alguien ve la base, no puede canjear nada. Con 129 bits de azar, SHA-256 simple es suficiente (no hace falta bcrypt). |
| **D7** | **Un solo código vigente por persona y oferta.** Si la app vuelve a pedir el código (reinstaló, se perdió la respuesta), se **revoca el anterior y se emite uno nuevo**. | Reintentar es siempre seguro y no hace falta `Idempotency-Key`: nunca quedan dos códigos válidos para lo mismo. |
| **D8** | **El canje es una sola sentencia `UPDATE … WHERE status = 'ISSUED' … RETURNING`.** | Dos canjes simultáneos del mismo código: **uno gana, el otro recibe 409**. No depende de bloqueos en Go. |
| **D9** | **Un código solo se canjea en la app destino de su oferta.** Si VisorUS POS recibe un código emitido para otra app, responde **404** (no revela que existe). | Cuando haya más apps, un código de "1 mes de App A" no sirve en App B. |
| **D10** | **Un correo canjea una misma oferta una sola vez** (índice único parcial). | Evita que alguien junte varios códigos (con varias cuentas) para acumular meses gratis en el mismo correo. |
| **D11** | **Bitácora `code_events` que solo crece**: emisión, revocación, validación fallida, canje, entrega. | Responde cualquier "¿qué pasó con este código?" y sirve para detectar abuso y medir. |
| **D12** | **Rutas con `/v1`.** | Los clientes son apps en tiendas que no puedes obligar a actualizar: un cambio incompatible va a `/v2` sin romper las versiones instaladas. |
| **D13** | **Errores en el formato de Emprende con Flow:** `{ "error": "CODE_ALREADY_REDEEMED", "message": "Este código ya fue canjeado." }` — `error` en MAYÚSCULAS, `message` en español listo para mostrar. | Las dos apps pueden mostrar el `message` sin traducir, y decidir por el `error`. |
| **D14** | **Sin panel web en la v1.** La administración del catálogo (apps, keys, campañas, ofertas) se hace con un **CLI** (`canjesctl`); la consulta de canjes, con los endpoints admin (JSON o CSV). | Un panel es una semana de trabajo; el CLI es una tarde y tú eres la única persona que lo opera. |
| **D15** | **La API confía en que la app emisora ya validó los puntos.** No conoce saldos ni reglas de puntos. | Separa responsabilidades: la economía de puntos es de cada plataforma emisora; esta API solo garantiza que el código sea único, de un solo uso y trazable. |

**Regla de negocio resultante:**
> Una **app emisora** (Emprende con Flow) lista las **ofertas** de las **campañas** activas en las que participa. Cuando una persona canjea, la app pide un **código** para esa oferta y esa persona (`holderRef`). La API emite un **código de 40 dígitos de un solo uso** —si había uno vigente para lo mismo, lo revoca— respetando el **límite por persona** y el **cupo de la oferta**, y guarda solo su **hash**. En la **app destino** (VisorUS POS), la persona escribe el código: la app lo **valida**, la persona **se registra** y la app lo **canjea con su correo**, una sola vez y solo en esa app. Cada paso queda en la **bitácora**. Tú consultas lo canjeado y marcas cuándo **entregaste** la promoción.

---

## 0.1 Actores

| Actor | Qué hace | Permisos (scopes) |
|---|---|---|
| **App Emprende con Flow** (Flutter) | Lista ofertas, emite códigos, consulta el estado de los códigos de la persona | `offers:read`, `codes:issue` |
| **App VisorUS POS** (o cualquier app destino) | Valida y canjea códigos con el correo del registro | `codes:redeem` |
| **Tú / el equipo** (CLI y endpoints admin) | Da de alta apps, keys, campañas y ofertas; consulta canjes; marca entregas; revoca | `admin` |
| **Persona usuaria** | Gana puntos en Emprende con Flow, canjea, descarga la app destino, escribe su código | — (nunca llama a la API directamente) |

---

## 0.2 Plan y Definition of Done

### Plan (una persona)

| Día | Entregable |
|---|---|
| **1** | Repositorio, `openapi.yaml`, migraciones, generador de código con Luhn y sus pruebas, CLI para crear app, key, campaña y oferta |
| **2** | Autenticación por API key con scopes, `GET /v1/offers`, `POST /v1/codes` (con revocación y cupo), `GET /v1/codes` |
| **3** | `POST /v1/codes/validate`, `POST /v1/codes/redeem`, endpoints admin (JSON + CSV), límite de intentos |
| **4** | Pruebas de integración contra Postgres real (incluida la de concurrencia), Docker, despliegue, semilla del piloto y prueba de punta a punta con las dos apps |

> **Dependencia con Emprende con Flow:** la versión del piloto se sube el **lunes 5 de octubre**. Si la API no está desplegada para entonces, la app sale con el canje local del SD v1.1 y la integración entra en una versión posterior.

### Definition of Done

- [ ] `GET /v1/offers` con la key de Emprende con Flow devuelve las ofertas de la campaña del piloto; con la key de VisorUS POS responde **403**.
- [ ] `POST /v1/codes` devuelve un código de **40 dígitos con Luhn válido**; en la base **no aparece** el código, solo su hash y los últimos 4.
- [ ] Pedir el código **otra vez** para la misma persona y oferta devuelve uno **nuevo**, y el anterior queda `REVOKED` (canjearlo responde **410**).
- [ ] Con el código ya canjeado, pedir otro para la misma persona y oferta responde **409 HOLDER_LIMIT_REACHED**.
- [ ] `validate` con un dígito cambiado responde **422 INVALID_CODE_FORMAT** sin consultar la base.
- [ ] `redeem` marca el código, guarda el correo en minúsculas y la fecha del consentimiento; un segundo `redeem` responde **409 CODE_ALREADY_REDEEMED**.
- [ ] **Prueba de concurrencia:** 20 goroutines canjean el mismo código a la vez → **exactamente una** recibe 200.
- [ ] Un código emitido para otra app destino responde **404** en VisorUS POS.
- [ ] El mismo correo no puede canjear dos códigos de la misma oferta → **409 EMAIL_ALREADY_REDEEMED**.
- [ ] `GET /v1/admin/codes?status=REDEEMED&format=csv` descarga lo canjeado; `fulfill` lo pasa a `FULFILLED`.
- [ ] Más de 10 validaciones fallidas por minuto desde la misma key e IP responden **429**.
- [ ] `/healthz` y `/readyz` responden; los logs salen en JSON con `request_id` y app cliente, **sin** códigos ni correos completos.

---

## 1. Requisitos

### 1.1 Funcionales

- **RF1** Listar las ofertas activas de las campañas donde la app que llama es emisora, con el estado de la persona en cada una (si tiene código vigente, si ya canjeó).
- **RF2** Emitir un código para `(oferta, persona)`: valida campaña activa, límite por persona y cupo; revoca el código vigente anterior si existe.
- **RF3** Listar los códigos de una persona **sin el código completo** (estado, últimos 4 dígitos, oferta, fechas), para que la app reconstruya "Mis canjes" tras reinstalar.
- **RF4** Validar un código sin cambiar nada: formato, existencia, que sea de la app que pregunta, que esté vigente. Devuelve qué promoción trae.
- **RF5** Canjear un código con correo, referencia del usuario en la app destino y fecha de consentimiento.
- **RF6** Admin: listar y exportar códigos por campaña, oferta y estado; marcar entregado; revocar; ver totales.
- **RF7** CLI: alta de apps, keys (se muestran una sola vez), campañas y ofertas; ejecutar migraciones.
- **RF8** (Opcional, se activa con una variable) Aviso a un webhook (Slack, Google Chat) en cada canje.

### 1.2 No funcionales

- **Volumen del piloto:** ~10 personas × 3 ofertas ≈ 30 códigos. **Diseñado para** decenas de miles de códigos por campaña sin cambiar el modelo.
- **Latencia:** < 100 ms P95 en todas las operaciones (son lecturas por índice o una sola escritura).
- **Consistencia:** un código **jamás** se canjea dos veces ni se reasigna; siempre hay como máximo un código vigente por persona y oferta.
- **Seguridad:** keys y códigos guardados solo como hash; TLS de la plataforma de despliegue; límite de intentos en validar y canjear.
- **Privacidad:** la API recibe de la app emisora solo un identificador opaco (`holderRef`), **nunca** nombre ni correo. El correo lo envía la app destino con consentimiento de la persona.
- **Disponibilidad:** una instancia basta para el piloto. El servicio no guarda estado en memoria salvo el límite de intentos, así que escalar es levantar otra instancia (§9).
- **Operación:** un solo binario, configuración por variables de entorno, migraciones versionadas.

### 1.3 Restricciones

- Una sola persona desarrolla y opera.
- Sin dependencias de pago obligatorias: Postgres puede ser el que la empresa ya tenga o uno administrado.
- Debe estar listo en días, no semanas.

---

## 2. Diseño de alto nivel

```
  ┌──────────────────────┐                         ┌──────────────────────┐
  │ App Emprende con Flow│                         │   App VisorUS POS    │
  │  (key: offers:read,  │                         │  (key: codes:redeem) │
  │        codes:issue)  │                         │                      │
  └─────────┬────────────┘                         └──────────┬───────────┘
            │ GET  /v1/offers                                 │ POST /v1/codes/validate
            │ POST /v1/codes        ┌──────────────────────┐  │ POST /v1/codes/redeem
            │ GET  /v1/codes        │      canjes-api      │  │
            └──────────────────────►│        (Go)          │◄─┘
                                    │                      │
   Tú ─ canjesctl / admin key ─────►│ middleware:          │──── webhook (opcional) ──► Slack /
        GET  /v1/admin/codes        │  request-id · logs · │                            Google Chat
        POST /v1/admin/codes/{id}/… │  recover · auth ·    │
                                    │  rate-limit          │
                                    │ handlers (oapi-codegen)
                                    │ servicio de códigos  │
                                    │ store (sqlc + pgx)   │
                                    └──────────┬───────────┘
                                               │
                                     ┌─────────▼──────────┐
                                     │     PostgreSQL     │
                                     │ client_apps · keys │
                                     │ campaigns · offers │
                                     │ codes · code_events│
                                     └────────────────────┘
```

### Flujo de punta a punta

```
 Persona            Emprende con Flow            canjes-api                 VisorUS POS
    │ abre "Canjes"        │                          │                          │
    │─────────────────────►│ GET /v1/offers?holderRef │                          │
    │                      │─────────────────────────►│ ofertas + estado         │
    │ toca "Canjear"       │                          │                          │
    │─────────────────────►│ (verifica puntos local)  │                          │
    │                      │ POST /v1/codes           │                          │
    │                      │─────────────────────────►│ emite, guarda hash       │
    │                      │◄─────── 201 código ──────│                          │
    │                      │ descuenta puntos local   │                          │
    │◄── código + "Descargar VisorUS POS" (tienda)    │                          │
    │                                                                            │
    │ instala la app, "Tengo un código de canje", pega el código                 │
    │───────────────────────────────────────────────────────────────────────────►│
    │                                                 │ POST /v1/codes/validate  │
    │                                                 │◄─────────────────────────│
    │                                                 │── 200 "1 mes gratis…" ──►│
    │ se registra; acepta el aviso "tu correo se usará para dar seguimiento"     │
    │───────────────────────────────────────────────────────────────────────────►│
    │                                                 │ POST /v1/codes/redeem    │
    │                                                 │◄─────────────────────────│
    │                                                 │ REDEEMED + correo        │
    │                                                 │──────── 200 ────────────►│
    │                                                 │── webhook al equipo      │
    │                                    Tú: GET /v1/admin/codes → activas la promoción → fulfill
```

> **Orden importante en Emprende con Flow:** primero `POST /v1/codes`, y **solo si responde 201** se descuentan los puntos en el teléfono. Si la red falla, no se descuenta nada y reintentar es seguro (D7).

---

## 3. Máquinas de estado

### 3.1 Código

```
               emisión
                 │
                 ▼
            ┌─────────┐   redeem (app destino, vigente)   ┌──────────┐   fulfill (admin)   ┌───────────┐
            │ ISSUED  │──────────────────────────────────►│ REDEEMED │───────────────────►│ FULFILLED │
            └────┬────┘                                   └──────────┘                     └───────────┘
                 │ reemisión para la misma persona y oferta
                 │ o revoke (admin)
                 ▼
            ┌─────────┐
            │ REVOKED │
            └─────────┘

  "Vencido" no es un estado guardado: un código ISSUED con expires_at < now() se trata como vencido
  en cada consulta (410 CODE_EXPIRED). Así no hace falta un proceso que corra cada noche.
```

| Transición | Quién | Condición |
|---|---|---|
| → `ISSUED` | app emisora | campaña activa ∧ oferta activa ∧ límite por persona ∧ cupo |
| `ISSUED → REDEEMED` | app destino | vigente ∧ app = destino de la oferta ∧ correo no usado en esa oferta |
| `ISSUED → REVOKED` | sistema (reemisión) o admin | — |
| `REDEEMED → FULFILLED` | admin | — |
| `REDEEMED → REVOKED` | **no permitido en v1** | Un canje no se deshace; si hay que compensar, se resuelve fuera de la API |

### 3.2 Campaña

`DRAFT → ACTIVE → CLOSED`. Además de su estado, una campaña tiene **ventanas**: `starts_at`/`ends_at` para **emitir** y `redeem_until` para **canjear** (p. ej. se emite hasta el 20 de octubre y se canjea hasta el 31).

---

## 4. Modelo de datos (PostgreSQL)

```sql
-- migrations/00001_init.sql  (goose)
-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- gen_random_uuid()

CREATE TABLE client_apps (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        text NOT NULL UNIQUE,                 -- 'emprende-con-flow', 'visorus-pos'
  name        text NOT NULL,
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  client_app_id  uuid NOT NULL REFERENCES client_apps(id),
  key_hash       text NOT NULL UNIQUE,              -- sha256 hex de la key completa
  key_prefix     text NOT NULL,                     -- 'ck_live_8F3A' para reconocerla en logs
  scopes         text[] NOT NULL,                   -- {offers:read,codes:issue} | {codes:redeem} | {admin}
  created_at     timestamptz NOT NULL DEFAULT now(),
  revoked_at     timestamptz
);

CREATE TABLE campaigns (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug           text NOT NULL UNIQUE,              -- 'ecf-piloto-2026-10'
  name           text NOT NULL,
  issuer_app_id  uuid NOT NULL REFERENCES client_apps(id),
  status         text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','ACTIVE','CLOSED')),
  code_format    text NOT NULL DEFAULT 'NUMERIC_40',
  starts_at      timestamptz NOT NULL,              -- desde cuándo se emite
  ends_at        timestamptz NOT NULL,              -- hasta cuándo se emite
  redeem_until   timestamptz NOT NULL,              -- hasta cuándo se canjea
  metadata       jsonb NOT NULL DEFAULT '{}',
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (starts_at < ends_at AND ends_at <= redeem_until)
);

CREATE TABLE offers (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id     uuid NOT NULL REFERENCES campaigns(id),
  target_app_id   uuid NOT NULL REFERENCES client_apps(id),   -- dónde se canjea
  slug            text NOT NULL,
  title           text NOT NULL,                    -- '1 mes gratis de VisorUS POS + 1 h de asesoría'
  description     text NOT NULL DEFAULT '',
  benefits        jsonb NOT NULL,                   -- [{"type":"FREE_MONTHS","value":1},{"type":"ADVISORY_HOURS","value":1}]
  cost_points     integer,                          -- informativo para la app emisora
  store_links     jsonb NOT NULL DEFAULT '{}',      -- {"ios":"https://apps.apple.com/…","android":"https://play.google.com/…"}
  max_per_holder  integer NOT NULL DEFAULT 1 CHECK (max_per_holder > 0),
  total_quota     integer CHECK (total_quota IS NULL OR total_quota > 0),
  sort_order      integer NOT NULL DEFAULT 0,
  active          boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (campaign_id, slug)
);

CREATE TABLE codes (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  offer_id            uuid NOT NULL REFERENCES offers(id),
  issuer_app_id       uuid NOT NULL REFERENCES client_apps(id),
  holder_ref          text NOT NULL,                -- profileId de Emprende con Flow (opaco)
  code_hash           text NOT NULL UNIQUE,         -- sha256 hex del código normalizado
  code_last4          char(4) NOT NULL,
  status              text NOT NULL DEFAULT 'ISSUED'
                      CHECK (status IN ('ISSUED','REDEEMED','FULFILLED','REVOKED')),
  issued_at           timestamptz NOT NULL DEFAULT now(),
  expires_at          timestamptz,                  -- = campaign.redeem_until al emitir
  redeemed_at         timestamptz,
  redeemed_by_app_id  uuid REFERENCES client_apps(id),
  redeemed_email      text,                         -- en minúsculas
  redeemer_ref        text,                         -- id del usuario en la app destino
  email_consent_at    timestamptz,
  consent_version     text,
  fulfilled_at        timestamptz,
  fulfilled_note      text,
  revoked_at          timestamptz,
  revoke_reason       text,                         -- 'REISSUED' | 'ADMIN' | …
  metadata            jsonb NOT NULL DEFAULT '{}'
);

-- Un solo código vigente por persona y oferta (D7)
CREATE UNIQUE INDEX ux_codes_one_active
  ON codes (offer_id, holder_ref) WHERE status = 'ISSUED';

-- Un correo canjea una oferta una sola vez (D10)
CREATE UNIQUE INDEX ux_codes_email_per_offer
  ON codes (offer_id, redeemed_email) WHERE redeemed_email IS NOT NULL;

CREATE INDEX ix_codes_holder       ON codes (issuer_app_id, holder_ref);
CREATE INDEX ix_codes_offer_status ON codes (offer_id, status);

CREATE TABLE code_events (
  id             bigserial PRIMARY KEY,
  code_id        uuid REFERENCES codes(id),         -- NULL en validaciones fallidas sin código encontrado
  offer_id       uuid REFERENCES offers(id),
  client_app_id  uuid REFERENCES client_apps(id),
  type           text NOT NULL,                     -- ISSUED | REVOKED | VALIDATE_OK | VALIDATE_FAILED
                                                    -- | REDEEMED | REDEEM_FAILED | FULFILLED
  detail         jsonb NOT NULL DEFAULT '{}',       -- {"reason":"CODE_EXPIRED"} — nunca el código completo
  ip             inet,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_code_events_code ON code_events (code_id, created_at);

-- +goose Down
DROP TABLE code_events, codes, offers, campaigns, api_keys, client_apps;
```

**Por qué así:**

- **`client_apps` es una sola tabla para emisoras y destinos.** Una app puede ser las dos cosas (VisorUS POS podría emitir códigos de otra app). Lo que decide qué puede hacer es la **key y sus scopes**, no la tabla.
- **`benefits` es `jsonb`** para sumar tipos (`DISCOUNT_PERCENT`, `EXTRA_USERS`, `EVENT_PASS`…) sin migración. La API no interpreta los beneficios: los guarda y los devuelve.
- **Las dos reglas de unicidad viven en índices parciales**, no en código Go. Aunque haya un bug en el servicio, la base no deja romperlas.
- **`code_events` nunca guarda el código completo ni el correo completo** (en `detail` va enmascarado: `u***@gmail.com`).

---

## 5. Contrato de la API

> Esto es el resumen; el contrato completo vive en `api/openapi.yaml` del repositorio (D2).

### 5.1 Autenticación y convenciones

- **Header:** `X-Api-Key: ck_live_<32 caracteres aleatorios>`. La API calcula el SHA-256, busca la key, verifica que no esté revocada, que la app esté activa y que tenga el **scope** del endpoint. Sin key → **401**; sin scope → **403**.
- **JSON** en `camelCase`; fechas **RFC 3339 en UTC**; IDs **UUID**.
- **Errores:** `{ "error": "CÓDIGO", "message": "Texto en español" }`.
- **Paginación** (listas admin): `?size=50&offset=0` → `{ "data": [...], "total": 123 }`.
- **`X-Request-Id`**: si la app lo manda se respeta; si no, se genera y se devuelve.

### 5.2 Endpoints de las apps

**`GET /v1/offers?holderRef={id}`** — scope `offers:read`

```jsonc
// 200
{ "campaign": { "slug": "ecf-piloto-2026-10", "endsAt": "2026-10-21T05:59:00Z" },
  "data": [
    { "offerId": "6b1f…", "slug": "visorus-pos-1m-asesoria",
      "title": "1 mes gratis de VisorUS POS + 1 h de asesoría",
      "description": "…",
      "benefits": [ { "type": "FREE_MONTHS", "value": 1 }, { "type": "ADVISORY_HOURS", "value": 1 } ],
      "costPoints": 1500,
      "targetApp": { "slug": "visorus-pos", "name": "VisorUS POS" },
      "storeLinks": { "ios": "https://apps.apple.com/mx/app/id…", "android": "https://play.google.com/store/apps/details?id=…" },
      "soldOut": false,
      "holder": { "activeCode": { "codeId": "…", "last4": "4821", "expiresAt": "…" },  // o null
                  "redeemedCount": 0, "remaining": 1 } } ] }
```

Si `holderRef` no viene, `holder` va en `null`.

**`POST /v1/codes`** — scope `codes:issue`

```jsonc
// req
{ "offerId": "6b1f…", "holderRef": "8f3a2c1e-…" }

// 201  — el código completo SOLO aparece aquí
{ "codeId": "c0de…",
  "code": "4821093755120864937102846619305527841930",
  "codeDisplay": "4821 0937 5512 0864 9371 0284 6619 3055 2784 1930",
  "last4": "1930",
  "offer": { "offerId": "6b1f…", "title": "…", "targetApp": { … }, "storeLinks": { … } },
  "expiresAt": "2026-11-01T05:59:59Z",
  "reissued": false }        // true si revocó un código vigente anterior
```

| Error | HTTP | Cuándo |
|---|:---:|---|
| `OFFER_NOT_FOUND` | 404 | No existe, está inactiva o es de una campaña de otra app emisora |
| `CAMPAIGN_NOT_ACTIVE` | 410 | Fuera de `starts_at`–`ends_at` o campaña no `ACTIVE` |
| `HOLDER_LIMIT_REACHED` | 409 | La persona ya canjeó `max_per_holder` veces esa oferta |
| `OFFER_SOLD_OUT` | 409 | Se alcanzó `total_quota` |
| `VALIDATION_ERROR` | 422 | Falta `offerId` o `holderRef`, o `holderRef` mide más de 128 caracteres |

**`GET /v1/codes?holderRef={id}`** — scope `codes:issue`

Lista los códigos de la persona **sin el código completo** (`codeId`, `last4`, `status`, `offer`, `issuedAt`, `redeemedAt`, `expiresAt`). Sirve para "Mis canjes" después de reinstalar: si un código está `ISSUED` y la persona perdió el número, la app vuelve a llamar `POST /v1/codes` y obtiene uno nuevo.

**`POST /v1/codes/validate`** — scope `codes:redeem` · **no cambia nada**

```jsonc
// req
{ "code": "4821 0937 5512 0864 9371 0284 6619 3055 2784 1930" }
// 200
{ "valid": true,
  "offer": { "title": "1 mes gratis de VisorUS POS + 1 h de asesoría",
             "benefits": [ … ] },
  "expiresAt": "2026-11-01T05:59:59Z" }
```

**`POST /v1/codes/redeem`** — scope `codes:redeem`

```jsonc
// req
{ "code": "4821…1930",
  "email": "Persona@Gmail.com",
  "redeemerRef": "pos-user-91822",          // id del usuario recién registrado en VisorUS POS
  "emailConsentAt": "2026-10-09T18:22:10Z", // cuándo aceptó la alerta de seguimiento
  "consentVersion": "pos-canje-v1" }
// 200
{ "codeId": "c0de…", "status": "REDEEMED",
  "offer": { "title": "…", "benefits": [ … ] },
  "redeemedAt": "2026-10-09T18:22:13Z" }
```

**Errores de `validate` y `redeem`:**

| Error | HTTP | Cuándo | `message` sugerido |
|---|:---:|---|---|
| `INVALID_CODE_FORMAT` | 422 | No son 40 dígitos o falla el Luhn (**sin consultar la base**) | "Revisa el código: parece que hay un dígito mal escrito." |
| `CODE_NOT_FOUND` | 404 | No existe **o** es de otra app destino | "Este código no es válido para esta app." |
| `CODE_ALREADY_REDEEMED` | 409 | Ya está `REDEEMED` o `FULFILLED` | "Este código ya fue canjeado." |
| `EMAIL_ALREADY_REDEEMED` | 409 | Ese correo ya canjeó esta oferta (solo `redeem`) | "Este correo ya recibió esta promoción." |
| `CODE_REVOKED` | 410 | Se reemitió o se revocó | "Este código ya no es válido. Pide uno nuevo en Emprende con Flow." |
| `CODE_EXPIRED` | 410 | Pasó `expires_at` | "Este código venció." |
| `VALIDATION_ERROR` | 422 | Correo inválido, falta `emailConsentAt` | — |
| `TOO_MANY_ATTEMPTS` | 429 | Límite de intentos (§6.6) | "Demasiados intentos. Espera un minuto." |

### 5.3 Endpoints admin — scope `admin`

| Endpoint | Qué hace |
|---|---|
| `GET /v1/admin/codes?campaign=&offer=&status=&size=&offset=&format=json\|csv` | Lista y exporta: oferta, `last4`, `holderRef`, estado, fechas, correo, `redeemerRef` |
| `GET /v1/admin/codes/{id}/events` | Bitácora de un código |
| `POST /v1/admin/codes/{id}/fulfill` `{ "note": "Activado en POS el 10-oct" }` | `REDEEMED → FULFILLED` |
| `POST /v1/admin/codes/{id}/revoke` `{ "reason": "…" }` | `ISSUED → REVOKED` |
| `GET /v1/admin/stats?campaign=` | Por oferta: emitidos, vigentes, canjeados, entregados, revocados, validaciones fallidas |

### 5.4 Operación (sin autenticación)

`GET /healthz` (el proceso vive) · `GET /readyz` (hay conexión con Postgres).

---

## 6. Reglas finas

### 6.1 Generación y verificación del código (`internal/codes`)

```go
// Formato NUMERIC_40: 39 dígitos aleatorios + 1 dígito verificador Luhn.
func Generate() (string, error) {
	const n = 39
	buf := make([]byte, 0, n+1)
	ten := big.NewInt(10)
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, ten) // crypto/rand: uniforme y seguro
		if err != nil {
			return "", err
		}
		buf = append(buf, byte('0'+d.Int64()))
	}
	return string(append(buf, luhnCheckDigit(buf))), nil
}

// Dígito que hace válido el número completo al agregarse a la derecha.
func luhnCheckDigit(payload []byte) byte {
	sum, double := 0, true // el último dígito del payload queda en posición par al agregar el verificador
	for i := len(payload) - 1; i >= 0; i-- {
		d := int(payload[i] - '0')
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return byte('0' + (10-sum%10)%10)
}

// Acepta espacios y guiones; devuelve los 40 dígitos o error.
func Normalize(input string) (string, error) {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			// se ignoran
		default:
			return "", ErrInvalidFormat
		}
	}
	s := b.String()
	if len(s) != 40 || luhnCheckDigit([]byte(s[:39])) != s[39] {
		return "", ErrInvalidFormat
	}
	return s, nil
}

func Hash(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
```

- **Colisiones:** con 10^39 combinaciones no ocurren en la práctica; aun así, si el `INSERT` choca con `code_hash` único, se genera otro y se reintenta (máximo 3 veces).
- **Formato por campaña (para crecer):** `campaigns.code_format` decide el generador. Hoy solo existe `NUMERIC_40`; agregar, por ejemplo, `ALNUM_16` (16 caracteres sin O/0 ni I/1, en grupos de 4) es una función nueva en `internal/codes` y nada más. Como en la base solo se guarda el hash, **formatos distintos conviven** sin cambiar el esquema.

### 6.2 Emisión (`POST /v1/codes`) — una transacción

```sql
-- name: LockOfferForIssue :one
SELECT o.*, c.status AS campaign_status, c.starts_at, c.ends_at, c.redeem_until, c.issuer_app_id
FROM offers o JOIN campaigns c ON c.id = o.campaign_id
WHERE o.id = $1 AND o.active
FOR UPDATE OF o;
```

1. `BEGIN`
2. Bloquear la oferta (consulta de arriba). Si no existe, está inactiva o `issuer_app_id ≠ app que llama` → `404 OFFER_NOT_FOUND`.
3. Campaña `ACTIVE` y `now()` entre `starts_at` y `ends_at`; si no → `410 CAMPAIGN_NOT_ACTIVE`.
4. `redeemed = COUNT(*) WHERE offer_id AND holder_ref AND status IN ('REDEEMED','FULFILLED')`; si `redeemed ≥ max_per_holder` → `409 HOLDER_LIMIT_REACHED`.
5. Revocar el vigente: `UPDATE codes SET status='REVOKED', revoked_at=now(), revoke_reason='REISSUED' WHERE offer_id=$1 AND holder_ref=$2 AND status='ISSUED' RETURNING id` → evento `REVOKED` si hubo.
6. Si `total_quota` no es nulo: `used = COUNT(*) WHERE offer_id AND status IN ('ISSUED','REDEEMED','FULFILLED')`; si `used ≥ total_quota` → `409 OFFER_SOLD_OUT`. (El código recién revocado ya no cuenta, así que reemitir nunca agota el cupo.)
7. Generar código, `INSERT` con `expires_at = redeem_until`, evento `ISSUED`.
8. `COMMIT` y responder **201** con el código completo.

> El bloqueo `FOR UPDATE` sobre la oferta serializa las emisiones **de esa oferta**. Con decenas de emisiones por minuto no se nota; es lo que garantiza que el cupo no se pase por dos emisiones simultáneas.

### 6.3 Canje (`POST /v1/codes/redeem`) — una sola sentencia

```sql
-- name: RedeemCode :one
UPDATE codes c
SET status             = 'REDEEMED',
    redeemed_at        = now(),
    redeemed_by_app_id = @app_id,
    redeemed_email     = @email,
    redeemer_ref       = @redeemer_ref,
    email_consent_at   = @consent_at,
    consent_version    = @consent_version
FROM offers o
WHERE c.code_hash = @code_hash
  AND c.status    = 'ISSUED'
  AND o.id        = c.offer_id
  AND o.target_app_id = @app_id
  AND (c.expires_at IS NULL OR c.expires_at > now())
RETURNING c.id, c.offer_id, o.title, o.benefits, c.redeemed_at;
```

- **Si devuelve una fila:** evento `REDEEMED`, webhook (fuera de la transacción, sin bloquear la respuesta) y **200**.
- **Si no devuelve nada:** se consulta el código por hash para explicar por qué: no existe o es de otra app → 404; `REDEEMED`/`FULFILLED` → 409; `REVOKED` → 410; vencido → 410. Evento `REDEEM_FAILED` con el motivo.
- **Si falla por `ux_codes_email_per_offer`** (error Postgres `23505` en ese índice) → `409 EMAIL_ALREADY_REDEEMED`.
- **El correo** se normaliza: `strings.TrimSpace` + `strings.ToLower`, y se valida con `net/mail.ParseAddress`.

### 6.4 Validación (`POST /v1/codes/validate`)

Normaliza y verifica Luhn → busca por hash → aplica las mismas condiciones que el canje **sin escribir nada** (salvo el evento `VALIDATE_OK`/`VALIDATE_FAILED`). Que un código sea válido al validar **no garantiza** que siga válido al canjear (alguien pudo canjearlo en medio); por eso el canje vuelve a verificar todo en la sentencia atómica.

### 6.5 Autenticación (`internal/httpapi/middleware`)

```go
func RequireScope(store KeyStore, scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("X-Api-Key")
			if raw == "" {
				writeError(w, 401, "UNAUTHORIZED", "Falta la API key.")
				return
			}
			key, err := store.FindActiveKey(r.Context(), sha256Hex(raw)) // une con client_apps.active
			if err != nil {
				writeError(w, 401, "UNAUTHORIZED", "API key inválida.")
				return
			}
			if !slices.Contains(key.Scopes, scope) {
				writeError(w, 403, "FORBIDDEN", "Esta app no tiene permiso para esta operación.")
				return
			}
			next.ServeHTTP(w, r.WithContext(withClient(r.Context(), key)))
		})
	}
}
```

Las keys se crean con el CLI (`canjesctl keys create --app visorus-pos --scopes codes:redeem`), que **muestra la key una sola vez** y guarda solo su hash. Las keys activas se pueden tener en memoria por 60 segundos para no consultar la base en cada petición.

### 6.6 Límite de intentos

- `validate` y `redeem`: **10 intentos fallidos por minuto por (key, IP)**; `POST /v1/codes`: **30 por minuto por key**.
- v1: en memoria con `golang.org/x/time/rate` (una instancia).
- Con 129 bits de azar nadie adivina un código por fuerza bruta; el límite existe para frenar abusos y errores de las apps (un bucle que reintenta sin parar).

### 6.7 Webhook opcional

Si existe `NOTIFY_WEBHOOK_URL`, cada canje manda en segundo plano: `"✅ Canje: VisorUS POS · 1 mes + asesoría · ***1930 · p***@gmail.com"`. Con timeout de 3 segundos y sin reintentos: es un aviso, no la fuente de verdad (la fuente es `GET /v1/admin/codes`).

---

## 7. Repositorio

### 7.1 Nombre

| Opción | A favor | En contra |
|---|---|---|
| **`canjes-api`** ✅ | Corto, en español como el resto de tus proyectos, describe exactamente lo que hace y **no está atado** a una app ni a una campaña | — |
| `redeem-hub` | En inglés, suena a plataforma | Menos claro para el equipo |
| `promo-codes-api` | Muy descriptivo | Se queda corto si mañana emite pases a eventos o descuentos |

**Recomendado: `canjes-api`**. Módulo Go `github.com/<organización>/canjes-api`, binario del servidor `canjes-api` y CLI `canjesctl`.

### 7.2 Estructura

```
canjes-api/
├── api/
│   └── openapi.yaml               ← contrato, fuente de verdad (D2)
├── cmd/
│   ├── api/main.go                ← servidor HTTP
│   └── canjesctl/main.go          ← CLI: migrate, apps, keys, campaigns, offers
├── internal/
│   ├── config/                    ← variables de entorno
│   ├── codes/                     ← generador, Luhn, normalización, hash (sin dependencias de BD)
│   ├── service/                   ← emisión, validación, canje, admin (reglas de §6)
│   ├── httpapi/
│   │   ├── gen/                   ← código generado por oapi-codegen (no se edita)
│   │   ├── handlers.go            ← implementa la interfaz generada
│   │   ├── errors.go              ← mapeo de errores de dominio → { error, message }
│   │   └── middleware/            ← request-id, logging, recover, auth, rate-limit
│   ├── store/
│   │   ├── queries/*.sql          ← SQL a mano
│   │   └── db/                    ← código generado por sqlc (no se edita)
│   └── notify/                    ← webhook
├── migrations/                    ← goose: 00001_init.sql, …
├── seeds/ecf-piloto-2026-10.sql   ← apps, campaña y ofertas del piloto
├── docs/SYSTEM_DESIGN.md          ← este documento
├── Dockerfile                     ← multi-stage → imagen distroless (~20 MB)
├── Makefile                       ← generate, test, run, migrate, docker
├── sqlc.yaml · oapi-codegen.yaml · .env.example · README.md
└── go.mod
```

**Regla de dependencias:** `httpapi → service → store`, y `codes` no depende de nadie. Los handlers no escriben SQL; el servicio no conoce HTTP.

### 7.3 Configuración (`.env.example`)

```
PORT=8080
DATABASE_URL=postgres://user:pass@host:5432/canjes?sslmode=require
LOG_LEVEL=info
RATE_LIMIT_REDEEM_PER_MIN=10
RATE_LIMIT_ISSUE_PER_MIN=30
NOTIFY_WEBHOOK_URL=            # opcional
```

### 7.4 Pruebas

| Tipo | Qué cubre | Herramienta |
|---|---|---|
| Unitarias | Generador (40 dígitos, Luhn válido), `Normalize` (espacios, guiones, letras, un dígito cambiado, dos invertidos), mapeo de errores | `go test` |
| Integración | Emisión con reemisión, límite por persona, cupo; canje y cada error; índice de correo; endpoints admin | `testcontainers-go` con Postgres real |
| **Concurrencia** | 20 goroutines canjean el mismo código → exactamente un 200; 20 emisiones contra cupo 5 → exactamente 5 | ″ |
| Contrato | Las respuestas cumplen `openapi.yaml` | validación con `kin-openapi` en las pruebas de integración |

### 7.5 Despliegue

- **Imagen Docker** multi-stage: compila con `CGO_ENABLED=0` y copia el binario a `gcr.io/distroless/static`.
- **Migraciones:** `canjesctl migrate up` como paso previo del despliegue (o al arrancar, con el bloqueo que trae `goose`, mientras haya una sola instancia).
- **Dónde:** en la plataforma donde la empresa ya despliega contenedores (Cloud Run, Fly.io, un VPS con Docker…). Postgres: el que ya tengan, o uno administrado. El dominio debe tener HTTPS (p. ej. `canjes.<dominio-de-la-empresa>`).
- **Respaldo:** el respaldo diario del Postgres administrado basta; el dato valioso es la tabla `codes`.

---

## 8. Casos borde

1. **La respuesta de emisión se pierde (sin señal).** La app reintenta → código nuevo, el anterior queda `REVOKED`. Los puntos solo se descuentan en el teléfono tras un 201.
2. **La persona reinstala Emprende con Flow y pierde su código.** `GET /v1/codes` muestra que tiene uno `ISSUED`; la app pide otro y el anterior se revoca.
3. **Valida, se registra y alguien canjea el código en medio** (por ejemplo, lo compartió). El canje responde 409 y la app destino muestra el mensaje con el contacto de soporte.
4. **Se registra con un correo que ya canjeó esa oferta.** `409 EMAIL_ALREADY_REDEEMED`. La cuenta en la app destino sí se crea, pero sin promoción.
5. **Escribe el código con espacios, guiones o pegado con un salto de línea.** Se normaliza; un salto de línea al final se recorta con `strings.TrimSpace` antes de normalizar.
6. **Escribe un dígito mal.** El Luhn lo detecta: 422 inmediato, sin tocar la base.
7. **Se termina la campaña con códigos sin canjear.** Siguen válidos hasta `redeem_until`; después responden 410.
8. **La key de Emprende con Flow se filtra** (va dentro de la app). Alguien podría emitir códigos con `holderRef` inventados. Mitigación del piloto: `total_quota` por oferta (p. ej. 20), revisar `stats` a diario y **revocar la key** (`canjesctl keys revoke`) si algo se ve raro. La solución de fondo está en §10.
9. **Dos instancias del servidor.** Todo lo crítico vive en Postgres; solo el límite de intentos se vuelve "por instancia" (aceptable hasta que se mueva a Redis).

---

## 9. Escala, observabilidad, seguridad y trade-offs

**Escala.** El piloto genera ~30 códigos. El modelo aguanta cientos de miles por campaña con los índices de §4: todas las operaciones son búsquedas por índice único o una escritura. El primer límite real sería el bloqueo por oferta en emisiones masivas (miles por segundo de la misma oferta), muy lejos de cualquier uso previsible.

**Observabilidad.**

- **Logs JSON** (`slog`) por petición: `request_id`, `client_app`, ruta, estado HTTP, latencia, código de error. **Nunca** el código completo, la key ni el correo completo.
- **Bitácora `code_events`**: la historia completa de cada código, consultable por admin.
- **`/v1/admin/stats`**: el tablero del piloto.
- **Alertas (cuando haya monitoreo):** pico de `VALIDATE_FAILED`/`REDEEM_FAILED` (abuso o bug en una app), errores 5xx, `/readyz` caído.
- **Para crecer:** métricas y trazas con OpenTelemetry, el mismo estándar de Emprende con Flow.

**Seguridad.**

| Riesgo | Mitigación |
|---|---|
| Key extraída de una app móvil | Scopes mínimos por app, cupo por oferta, revocación inmediata con el CLI |
| Filtración de la base | Códigos y keys solo como hash; correos solo de quienes canjearon |
| Fuerza bruta | 129 bits + Luhn + límite de intentos |
| Canje doble | Sentencia atómica + índices únicos parciales |
| Datos personales | La API nunca recibe nombre ni correo de la app emisora; el correo llega con fecha y versión del consentimiento |

**Trade-offs elegidos.**

| Decisión | Ganas | Pagas |
|---|---|---|
| Solo hash del código | Una base filtrada no sirve para canjear | El código no se puede volver a mostrar; se resuelve reemitiendo (D7) |
| 40 dígitos numéricos | Teclado numérico en el teléfono, cero ambigüedad entre letras y números | Largo para escribir a mano: la app emisora debe tener **copiar** y la destino debe aceptar **pegar** |
| Sin `Idempotency-Key` | Menos código y menos estado | Un reintento genera un código nuevo (inofensivo: el anterior se revoca) |
| Límite de intentos en memoria | Cero infraestructura extra | Con varias instancias, el límite es por instancia |
| CLI en vez de panel | Listo en una tarde | Solo tú lo operas |
| La API confía en la app emisora | Separación limpia: la API no sabe de puntos | Una key filtrada puede emitir dentro del cupo (§8, borde 8) |

---

## 10. Cómo crece (lo que revisaría al crecer)

| Cuando… | Se hace | Qué cambia |
|---|---|---|
| **Emprende con Flow tenga los endpoints de gamificación de S4** | El **backend** de Emprende con Flow emite los códigos al canjear un beneficio, **de servidor a servidor**, y la key sale de la app móvil | Nada en esta API: es otra key con `codes:issue` |
| Se quiera eliminar el riesgo de la key en la app | La API acepta el **JWT de la persona** emitido por Emprende con Flow, lo verifica con su llave pública y toma `holderRef` del token, no del cuerpo | Un middleware nuevo; el contrato agrega un header |
| VisorUS POS quiera aplicar la promoción solo | Su **backend** llama a `redeem` y activa el mes gratis en el mismo paso; se marca `FULFILLED` automáticamente | Un parámetro `autoFulfill` o un webhook de salida por app |
| Haya más apps destino | Alta con el CLI: app, key, ofertas | Nada en el código |
| Haya más tipos de beneficio | Nuevos `type` en `benefits` | Nada en el esquema |
| Se quiera un código corto | Nuevo `code_format` por campaña | Una función en `internal/codes` |
| El equipo de ventas quiera ver los canjes | Panel web de solo lectura sobre los endpoints admin, o un tablero (Metabase) sobre una vista de Postgres | Nada en la API |
| Haya varias instancias | Límite de intentos en Redis | Solo el middleware |
| Se necesite borrar correos por privacidad | Job que anonimiza `redeemed_email` N meses después de `redeem_until` | Una tarea programada |

---

## 11. Decisiones abiertas

| # | Decisión | Default | Quién |
|:-:|---|---|---|
| **A1** | Dónde se despliega y qué Postgres se usa | La plataforma de contenedores que ya use la empresa | Uriel / equipo |
| **A2** | Formato del código: 40 dígitos o uno corto alfanumérico | **40 dígitos** (D5); se puede cambiar por campaña | Uriel |
| **A3** | Fecha límite de canje (`redeem_until`) del piloto | **31 de octubre de 2026, 23:59 (centro de México)** | Uriel |
| **A4** | `total_quota` por oferta en el piloto | **20** por oferta (el doble de participantes) | Uriel |
| **A5** | Canal del webhook de avisos | Ninguno al inicio; consultar admin a diario | Uriel |
| **A6** | Cuánto tiempo se guardan los correos | 12 meses después del fin de la campaña | Empresa (área legal) |
| **A7** | Texto de la alerta de VisorUS POS y su `consentVersion` | *"Tu correo se usará para dar seguimiento a tu canje."* · `pos-canje-v1` | Empresa |

---

## Integración en las apps (resumen)

**Emprende con Flow (Flutter)**

1. Al abrir Canjes: `GET /v1/offers?holderRef={profileId}` (se puede cachear unos minutos).
2. Al confirmar: verifica los puntos en local → `POST /v1/codes` → **solo con 201** registra el movimiento de puntos local con el `codeId`.
3. Muestra `codeDisplay` con botón **Copiar** y el botón **Descargar** según el sistema (`storeLinks`).
4. "Mis canjes": `GET /v1/codes?holderRef=` + lo guardado en local.

**VisorUS POS**

1. Botón **"Tengo un código de canje"** → campo que acepta pegar (teclado numérico, admite espacios).
2. `POST /v1/codes/validate` → muestra la promoción ("1 mes gratis + 1 hora de asesoría").
3. Registro normal → alerta: *"Tu correo se usará para dar seguimiento a tu canje"* → al aceptar, guarda la hora.
4. `POST /v1/codes/redeem` con correo, id del usuario y hora del consentimiento.
5. Si el canje falla después del registro, la cuenta se conserva y se muestra el `message` con el contacto de soporte.

---

## Referencias

- `system-design/SD_Piloto_Canje_Promociones_Externas.md` (v1.1) — el piloto de Emprende con Flow que consume esta API; se actualizará cuando la API esté lista.
- `contratos-api/openapi-s4.yaml` — formato de errores (`ApiError`) y convenciones que se reutilizan aquí.
- Documentación de `sqlc`, `pgx`, `goose`, `oapi-codegen` (servidor `std-http`), `testcontainers-go` y `golang.org/x/time/rate`.
- Algoritmo de Luhn (ISO/IEC 7812-1) para el dígito verificador.
