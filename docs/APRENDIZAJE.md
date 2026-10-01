# Ruta de aprendizaje — Go aprendiendo con `canjes-api`

Construirás la API y aprenderás Go al mismo tiempo. Esta guía te dice **qué estudiar, en qué orden y para qué playbook lo necesitas**, sin pedirte que aprendas todo Go antes de empezar.

> **Consejo:** no intentes memorizar. Lee el tema, mira el código que Claude escribió en ese playbook y pídele: *"explícame este archivo línea por línea, como si no supiera Go"*. Aprender sobre tu propio código es mucho más rápido que con ejemplos genéricos.

---

## 1. Recursos base (gratuitos)

| Recurso | Para qué | Enlace |
|---|---|---|
| **A Tour of Go** | El recorrido oficial e interactivo. Es la base de todo. | https://go.dev/tour |
| **Go by Example** | Ejemplos cortos por tema; ideal para consultar. | https://gobyexample.com |
| **Effective Go** | Cómo escribir Go "como se debe". Léelo cuando ya hayas hecho el Tour. | https://go.dev/doc/effective_go |
| **Documentación de la librería estándar** | Referencia de `net/http`, `crypto/rand`, `log/slog`… | https://pkg.go.dev/std |
| **Tutorial: crear un módulo** | Cómo se organiza un proyecto Go. | https://go.dev/doc/tutorial/create-module |
| **Tutorial: acceder a una BD** | Primer contacto con SQL desde Go. | https://go.dev/doc/tutorial/database-access |
| **Learn SQL / PostgreSQL** | Repaso de SQL (SELECT, UPDATE, índices, transacciones). | https://www.postgresql.org/docs/current/tutorial.html |

---

## 2. Ruta por etapas

Cada etapa coincide con uno o varios playbooks. **Tiempo estimado de estudio** = lo mínimo para entender lo que se construye, no para dominarlo.

| Etapa | Antes de los playbooks | Qué estudiar | Tiempo |
|---|---|---|---|
| **0. Herramientas** | 00 | Qué es una terminal, Git básico (`clone`, `commit`, `push`), qué es Docker y un contenedor, qué es una variable de entorno. | 2 h |
| **1. Lo esencial de Go** | 01 | Tour of Go: *Basics* (paquetes, funciones, variables, tipos, `for`, `if`, `switch`, `defer`). Qué son `go.mod`, `go run`, `go build`, `go test`. | 3 h |
| **2. Tipos y datos** | 02, 03 | Tour: *More types* (structs, punteros, slices, maps). JSON en Go (*Go by Example: JSON*). Qué es una API REST, verbos HTTP, códigos 200/201/401/403/404/409/410/422/429/500. Qué es OpenAPI. SQL: `CREATE TABLE`, índices, `UNIQUE`, transacciones. | 4 h |
| **3. Funciones, errores y pruebas** | 04 | Tour: *Methods and interfaces* (métodos, interfaces, **errores**). *Go by Example*: Errors, Custom Errors, Testing. Pruebas con tabla (*table-driven tests*). Qué es un hash (SHA-256) y por qué no se "deshace". Algoritmo de Luhn. | 4 h |
| **4. Servidor HTTP** | 05 | `net/http`: `Handler`, `HandlerFunc`, `ServeMux`. Middleware (una función que envuelve a otra). `context.Context` (*Go by Example: Context*). Logs estructurados con `log/slog`. Señales y apagado ordenado. | 4 h |
| **5. Base de datos en Go** | 03, 06, 08 | `pgx` y `pgxpool` (pool de conexiones). Qué hace `sqlc`. Transacciones (`BEGIN/COMMIT/ROLLBACK`) y `SELECT … FOR UPDATE`. | 3 h |
| **6. Seguridad básica** | 07 | API keys vs contraseñas. Por qué se guarda el hash. Encabezados HTTP. Scopes/permisos. | 2 h |
| **7. Concurrencia** | 09, 10, 12 | Tour: *Concurrency* (goroutines, channels, `sync.WaitGroup`). Qué es una condición de carrera y `go test -race`. Por qué la BD resuelve la concurrencia del canje. | 3 h |
| **8. Entregar** | 13, 14, 15 | Dockerfile multi-stage, imagen vs contenedor, registro de imágenes. Qué es CI. Variables secretas. HTTPS y dominios. Cómo leer logs en la nube. | 4 h |

Total aproximado: **~30 h** de estudio repartidas a lo largo de los playbooks.

---

## 3. Conceptos de Go que verás en este proyecto (cheat-sheet)

| Concepto | Qué es, en una línea | Dónde aparece |
|---|---|---|
| **package** | Una carpeta de archivos `.go` que se usan juntos. | Cada carpeta en `internal/` |
| **module** (`go.mod`) | El proyecto completo y la lista de librerías que usa. | Raíz del repo |
| **`internal/`** | Carpeta especial: su código solo lo puede usar este proyecto. | `internal/*` |
| **struct** | Un "formulario" con campos con nombre (`Offer{Title, CostPoints}`). | En todos lados |
| **puntero** (`*T`) | Una referencia a un valor en lugar de una copia. | Columnas que pueden ser `NULL` |
| **slice** (`[]T`) | Una lista que crece. | Listas de ofertas, scopes |
| **map** (`map[K]V`) | Un diccionario clave → valor. | Caché de keys, rate-limit |
| **error** | Go no usa excepciones: las funciones devuelven `(resultado, error)` y tú revisas `if err != nil`. | En todos lados |
| **`errors.Is`** | Pregunta "¿este error es de tal tipo?", aunque venga envuelto. | `httpapi/errors.go` |
| **interface** | Un "contrato" de métodos; cualquier tipo que los tenga lo cumple. | `KeyStore`, interfaz generada del servidor |
| **`context.Context`** | Mochila que viaja con cada petición: tiempo límite, cancelación, datos como el `request_id`. | Todas las funciones de servicio y BD |
| **goroutine** (`go f()`) | Ejecutar algo en paralelo, muy barato. | Webhook en segundo plano, prueba de concurrencia |
| **`defer`** | "Ejecuta esto al salir de la función" (cerrar, liberar). | Transacciones, archivos |
| **middleware** | Función que envuelve a un handler para hacer algo antes/después. | `internal/httpapi/middleware` |
| **código generado** | Archivos `.go` que escribe una herramienta a partir del YAML o del SQL. | `gen/`, `db/` |
| **build tag** (`//go:build integration`) | Etiqueta que hace que unos archivos solo se compilen si la pides. | `test/integration` |
| **`go:embed`** | Mete archivos (las migraciones) dentro del binario. | `migrations/embed.go` |

---

## 4. Glosario del dominio (no técnico)

| Término | Significado en este proyecto |
|---|---|
| **App emisora** | La app que entrega códigos (Emprende con Flow). |
| **App destino** | La app donde se usan (VisorUS POS). |
| **Campaña** | Un periodo de promoción (el piloto de octubre). Tiene fechas para emitir y para canjear. |
| **Oferta** | Una promoción concreta dentro de una campaña ("1 mes gratis + 1 h de asesoría"). |
| **Código** | El "boleto" de 40 dígitos. Se entrega una vez y se usa una vez. |
| **holderRef** | Un identificador de la persona en la app emisora, sin nombre ni correo. |
| **Emitir** | Crear un código para una persona y oferta. |
| **Validar** | Preguntar "¿este código sirve?" sin gastarlo. |
| **Canjear** | Gastar el código y dejar registrado con qué correo. |
| **Entregar (fulfill)** | Marcar que tú ya activaste la promoción a esa persona. |
| **Revocar** | Anular un código antes de que se use. |
| **API key** | La "llave" que identifica a cada app frente a la API. |
| **Scope** | Qué puede hacer cada llave (leer ofertas, emitir, canjear, administrar). |
| **Hash** | La "huella" de un dato: sirve para comparar, pero no para reconstruir el dato original. |
| **Luhn** | Un dígito final que delata errores de tecleo (el mismo truco de las tarjetas de crédito). |
| **Migración** | Un archivo que crea o cambia tablas de la base de datos, con número de versión. |
| **Bitácora (`code_events`)** | El historial de todo lo que le pasó a cada código. |

---

## 5. Cómo estudiar con Claude

- `Explícame <concepto> con un ejemplo de este repo.`
- `Hazme 5 preguntas para comprobar que entendí el playbook 04.`
- `¿Qué pasaría si quito este defer?` → aprendes rompiendo cosas en una rama.
- `Escribe una prueba que falle y explícame por qué falla.`
- Cuando termines un playbook, intenta **explicarle a Claude** lo que hiciste; pídele que te corrija.
