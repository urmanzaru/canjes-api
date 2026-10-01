# 16 · Operar el piloto

| | |
|---|---|
| **Objetivo** | Rutina diaria y guía de incidentes para el piloto de Emprende con Flow (7–20 de octubre de 2026, canje hasta el 31), y el cierre ordenado de la campaña. |
| **Prioridad** | 🔴 Crítico |
| **Fechas** | 7 – 31 de octubre de 2026 |
| **Depende de** | 15 |
| **Desbloquea** | Retrospectiva y siguiente versión (SD §10) |
| **Secciones del SD** | N4, §3, §5.3, §8 (casos borde), §9 (observabilidad), §10, §11 (A5, A6) |
| **Tiempo estimado** | 15 min diarios |

---

## Notas para ti (sin tecnicismos)

La API ya está abierta. Tu trabajo ahora es **atender la ventanilla**: una vez al día revisas qué se canjeó, activas la promoción en VisorUS POS a cada persona y lo marcas como entregado. Si algo raro pasa, aquí está qué hacer.

No necesitas programar nada en esta etapa. Todo son consultas con `curl` (o el script `scripts/admin.sh` si lo creaste en el 11) y comandos de `canjesctl`.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Leer logs JSON en la consola de tu plataforma | Logs Explorer (Cloud Run) o `docker compose logs -f api` (VPS) |
| `jq` para leer respuestas JSON en la terminal | https://jqlang.github.io/jq/tutorial/ |
| Qué es un *runbook* | Esta misma página es uno |

---

## Requisitos previos

- [ ] 15 completo; simulacro superado.
- [ ] Key `admin` de producción a mano (en el gestor de contraseñas), exportada en tu terminal como `ADMIN_KEY`.
- [ ] `export API=https://canjes.<dominio>`.

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-troubleshooting` | Cuando algo se comporte distinto a lo esperado: método, no adivinanzas. |
| `golang-observability` | Interpretar logs y decidir alertas. |
| `golang-security` | Respuesta a una key filtrada o a un patrón de abuso. |

---

## Rutina diaria (15 min)

```bash
# 1. ¿Está viva?
curl -s $API/readyz

# 2. Tablero de la campaña
curl -s -H "X-Api-Key: $ADMIN_KEY" "$API/v1/admin/stats?campaign=ecf-piloto-2026-10" | jq

# 3. Canjes pendientes de entregar
curl -s -H "X-Api-Key: $ADMIN_KEY" "$API/v1/admin/codes?campaign=ecf-piloto-2026-10&status=REDEEMED" | jq

# 4. Para cada uno: activa la promoción en VisorUS POS con el correo y luego márcalo entregado
curl -s -X POST -H "X-Api-Key: $ADMIN_KEY" -H 'Content-Type: application/json' \
  -d '{"note":"Activado en POS el 10-oct"}' "$API/v1/admin/codes/<codeId>/fulfill" | jq

# 5. (Viernes) Exporta el CSV completo y guárdalo en un lugar seguro
curl -s -H "X-Api-Key: $ADMIN_KEY" "$API/v1/admin/codes?campaign=ecf-piloto-2026-10&format=csv" -o canjes-$(date +%F).csv
```

**Señales a vigilar en el tablero:**

| Señal | Normal en el piloto | Revisar si… |
|---|---|---|
| `issued` | ≤ 10 personas × ofertas | Supera mucho a los participantes (posible key filtrada, ver incidente B) |
| `validateFailed` / `redeemFailed` | Pocos (errores de tecleo) | Decenas en un día (bug en una app o abuso) |
| `active` cerca del cupo | — | Quedan < 3 de cupo: decide si aumentarlo |
| `redeemed` sin `fulfilled` por > 1 día | 0 | Se te pasó alguna entrega |

---

## Guía de incidentes

### A. "Mi código no funciona" (persona usuaria)

1. Pide los **últimos 4 dígitos** y el correo con el que se registró.
2. Busca su código por `last4`: `curl -s -H "X-Api-Key: $ADMIN_KEY" "$API/v1/admin/codes?campaign=ecf-piloto-2026-10&size=500" | jq '.data[] | select(.last4=="1930")'` (o búscalo en el CSV).
3. Mira su historia: `GET /v1/admin/codes/<codeId>/events`.

| Lo que ves | Qué pasó | Qué hacer |
|---|---|---|
| `REVOKED` / `REISSUED` | La app le dio un código nuevo después | Que abra "Mis canjes" en Emprende con Flow; si perdió el número, la app pide otro |
| `REDEEMED` con otro correo | Lo compartió o lo canjeó otra persona | Caso de soporte humano (SD §8 borde 3) |
| `EMAIL_ALREADY_REDEEMED` en `REDEEM_FAILED` | Ese correo ya recibió la promoción | Es la regla D10; explicar |
| `CODE_EXPIRED` | Pasó el 31 de octubre | Fuera de plazo |
| Solo `VALIDATE_FAILED` con `CODE_NOT_FOUND` | Lo está escribiendo en otra app o mal | Que lo **pegue** en lugar de teclearlo |

### B. Se sospecha que la key de Emprende con Flow se filtró

La key viaja dentro de la app (SD §8 borde 8). **Revocarla rompe la app instalada de todas las personas** hasta que salga una actualización. Por eso, en orden:

1. **Contener sin romper:** baja el cupo de las ofertas o cierra la campaña temporalmente (`canjesctl campaigns close` y, cuando pase, reactivar). Los códigos ya emitidos siguen canjeables.
2. Revisa en `code_events` y en el CSV los `holderRef` sospechosos y **revoca** sus códigos sin canjear (`POST /v1/admin/codes/{id}/revoke`).
3. Solo si el abuso continúa: `canjesctl keys revoke`, crear una key nueva y publicar una actualización de la app.
4. A futuro: SD §10 (emisión servidor a servidor o JWT de la persona).

### C. Se filtró la key de VisorUS POS o la de admin

Revócala de inmediato (`canjesctl keys revoke --prefix …`, efectivo en ≤ 60 s), crea otra y entrégala por el gestor. Si es la del POS, coordina con su equipo: la app (o su backend) debe actualizarse.

### D. La API no responde / `/readyz` falla

1. Logs de la plataforma: ¿error al conectar a Postgres? ¿pánicos?
2. Estado de la base en la consola del proveedor.
3. Si fue un despliegue reciente: vuelve a la revisión anterior (playbook 15 §11).
4. Avisa a los equipos de las apps: mientras la API esté caída, Emprende con Flow **no descuenta puntos** (solo lo hace tras un 201) y VisorUS POS muestra el `message` de error.

### E. Se agotó una oferta antes de tiempo

Decide con el equipo. Para ampliar el cupo hoy no hay comando: se hace con un `UPDATE offers SET total_quota = … WHERE slug = …` en la base (anótalo en esta página) o se agrega `canjesctl offers update --quota` como mejora.

---

## Cierre de la campaña

| Fecha | Acción |
|---|---|
| **21 oct** (fin de emisión) | La API deja de emitir sola (`ends_at`). Opcional: `canjesctl campaigns close --slug ecf-piloto-2026-10`. Los códigos emitidos siguen canjeables. |
| **1 nov** (fin de canje) | Los códigos `ISSUED` responden `CODE_EXPIRED` sin hacer nada. Exporta el CSV final y entrega lo pendiente. |
| **Primera semana de nov** | Retrospectiva: métricas finales de `stats`, incidentes, qué cambiar (SD §10). |
| **Según A6** (p. ej. 12 meses tras el cierre) | Anonimizar `redeemed_email` (tarea programada propuesta en SD §10). Agéndalo hoy. |

---

## Cómo saber que quedó bien

- [ ] Todos los `REDEEMED` del día terminan en `FULFILLED` en ≤ 24 h.
- [ ] No hubo 5xx sin explicación.
- [ ] El CSV final está guardado en un lugar seguro y con acceso restringido.
- [ ] La retrospectiva quedó escrita (puede ser una sección nueva en el SD).

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-troubleshooting, golang-observability).
Estoy operando el piloto con docs/playbooks/16-operar-piloto.md. Este es el resultado de
/v1/admin/stats y estos son los eventos del código <…> (sin datos personales): <…>.
¿Qué está pasando y qué paso de la guía de incidentes aplica?
```

> No pegues correos completos ni keys en el chat; usa los valores enmascarados.

---

## Próximos pasos

- ➡️ **14 — CI**, si no se hizo.
- Revisar SD §10 con lo aprendido: emisión servidor a servidor, `autoFulfill`, panel de solo lectura, filtro por `last4`/`holderRef` en admin, comando `offers update`.
- Actualizar el SD con todo lo que cambió durante la implementación (`TRUSTED_PROXY_HOPS`, `INVALID_STATE`, decisiones A1–A7 tomadas).
