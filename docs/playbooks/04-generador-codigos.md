# 04 · Generador de códigos (Luhn + hash)

| | |
|---|---|
| **Objetivo** | Implementar `internal/codes`: generar códigos de 40 dígitos (39 aleatorios + Luhn), normalizarlos, calcular su hash, darles formato y enmascarar datos sensibles. Además, generar API keys. Todo con pruebas unitarias y *fuzzing*. |
| **Prioridad** | 🔴 Crítico |
| **Día sugerido** | Día 1 (jue 1 oct) |
| **Depende de** | 01 |
| **Desbloquea** | 06, 08 |
| **Secciones del SD** | D5, D6, §6.1, §8 (bordes 5 y 6) |
| **Tiempo estimado** | 2 h + 4 h de estudio |

---

## Notas para ti (sin tecnicismos)

Esta es la **imprenta de boletos**. Es la pieza más independiente del sistema: no sabe nada de bases de datos ni de internet, solo de números. Por eso es **el mejor lugar para empezar a aprender Go**: funciones pequeñas, entradas y salidas claras, y pruebas fáciles de leer.

Qué hace cada función:

- **Generar:** saca 39 dígitos al azar con el generador seguro del sistema (como lanzar un dado de 10 caras 39 veces, pero un dado imposible de predecir) y agrega un dígito final de control.
- **Dígito de control (Luhn):** el mismo truco de las tarjetas de crédito. Si alguien escribe mal un dígito, la cuenta "no cuadra" y la API lo rechaza **sin consultar la base de datos**.
- **Normalizar:** acepta el código con espacios, guiones o un salto de línea al pegar, y lo deja en 40 dígitos limpios.
- **Hash:** calcula la "huella" del código. En la base solo se guarda la huella; con ella se puede comprobar un código, pero no reconstruirlo.
- **Formato:** `4821 0937 5512 …` en grupos de 4 para que sea legible.
- **Enmascarar:** convierte `persona@gmail.com` en `p***@gmail.com` para logs y bitácora.

> **Dato curioso que conviene saber:** Luhn detecta **todos** los errores de un dígito y **casi todas** las inversiones de dos dígitos vecinos. La única que no detecta es cambiar `09` por `90` (o al revés). Es una limitación conocida del algoritmo; con 40 dígitos y la opción "pegar", el riesgo práctico es mínimo. Las pruebas lo reflejan.

---

## Qué estudiar para entenderlo

| Tema | Recurso |
|---|---|
| Funciones con varios valores de retorno y errores | https://go.dev/tour/basics/6 · https://gobyexample.com/errors |
| `strings.Builder`, `for … range` sobre un string, `byte` vs `rune` | https://gobyexample.com/strings-and-runes |
| Pruebas en Go (`go test`, `t.Run`, tablas de casos) | https://gobyexample.com/testing-and-benchmarking |
| *Fuzzing* (pruebas con entradas aleatorias) | https://go.dev/doc/tutorial/fuzz |
| Algoritmo de Luhn | https://es.wikipedia.org/wiki/Algoritmo_de_Luhn |
| Qué es un hash (SHA-256) y por qué no se "deshace" | Pídele a Claude: "explícame un hash como si tuviera 12 años" |
| `crypto/rand` vs `math/rand` | https://pkg.go.dev/crypto/rand |

---

## Requisitos previos

- [ ] Playbook 01 completo (`go build ./...` funciona).
- [ ] Leíste SD §6.1 (el código Go de referencia está ahí).

---

## Skills a utilizar

| Skill | Para qué |
|---|---|
| `golang-security` | `crypto/rand` (nunca `math/rand`), SHA-256 sin sal es suficiente con 129 bits de azar (D6), no registrar secretos. |
| `golang-testing` | Pruebas con tabla, subpruebas con nombre, *fuzzing*, cobertura. |
| `golang-safety` | Índices fuera de rango, conversiones `byte`/`int`, entradas vacías. |
| `golang-naming` | Nombres exportados claros (`Generate`, `Normalize`, `Hash`, `ErrInvalidFormat`). |

---

## Plan (paso a paso)

### 1. API pública del paquete

| Función | Comportamiento |
|---|---|
| `Generate() (string, error)` | 39 dígitos con `crypto/rand` + Luhn. Copiar SD §6.1. |
| `Normalize(input string) (string, error)` | `strings.TrimSpace` **primero** (borde 5), ignora espacios y guiones, rechaza cualquier otro carácter, exige 40 dígitos y Luhn válido. Si no: `ErrInvalidFormat`. |
| `Hash(normalized string) string` | SHA-256 en hexadecimal (64 caracteres). |
| `Last4(normalized string) string` | Últimos 4 dígitos. |
| `Display(normalized string) string` | Grupos de 4 separados por espacio. |
| `MaskEmail(email string) string` | `persona@gmail.com` → `p***@gmail.com`. Entradas raras no deben entrar en pánico. |
| `GenerateAPIKey() (key, prefix string, err error)` | `ck_live_` + 32 caracteres aleatorios (alfabeto base62 con `crypto/rand`); `prefix` = `ck_live_` + primeros 4 (para logs). |
| `var ErrInvalidFormat = errors.New("invalid code format")` | Error centinela. |

Archivos: `codes.go`, `luhn.go`, `apikey.go`, `mask.go`, `doc.go`.

### 2. Pruebas unitarias (`codes_test.go`), con tabla

- **Generate:** 1 000 veces → siempre 40 caracteres, solo dígitos, `Normalize` lo acepta, sin repetidos.
- **Normalize — acepta:** con espacios cada 4, con guiones, con `\n` al final, con espacios al inicio.
- **Normalize — rechaza:** vacío, 39 dígitos, 41 dígitos, con letra, con `+`, con emoji.
- **Un dígito cambiado:** para un código válido, cambia **cada posición** por cada uno de los otros 9 dígitos → **todas** las variantes se rechazan (360 casos).
- **Dos vecinos invertidos:** para cada par de posiciones vecinas con dígitos distintos, inviértelos → se rechaza, **excepto** cuando el par es `0`/`9` (documentar en el nombre del caso).
- **Hash:** estable (mismo valor dos veces), 64 caracteres hex, distinto para códigos distintos; valor conocido para una entrada fija.
- **Display / Last4 / MaskEmail:** casos normales y raros (`"@x.com"`, `"sin-arroba"`, `""`).
- **GenerateAPIKey:** prefijo correcto, longitud 40, sin repetidos en 1 000.

### 3. *Fuzzing* (`fuzz_test.go`)

`FuzzNormalize`: con cualquier string, `Normalize` **nunca** entra en pánico, y si devuelve sin error, el resultado tiene 40 dígitos y Luhn válido.

### 4. Ejemplo ejecutable (`example_test.go`)

`ExampleDisplay` con salida esperada: sirve de documentación y de prueba.

---

## Cómo saber que quedó bien

```bash
go test ./internal/codes -v -race
go test ./internal/codes -cover                         # objetivo: ≥ 95 %
go test ./internal/codes -run=^$ -fuzz=FuzzNormalize -fuzztime=30s
go vet ./internal/codes && make lint
```

- [ ] Todas las pruebas pasan; cobertura ≥ 95 %.
- [ ] 30 s de *fuzzing* sin fallos.
- [ ] El paquete **no importa** nada fuera de la librería estándar: `go list -deps ./internal/codes | grep '\.'` no muestra nada (los módulos externos llevan un punto, como `github.com/...`).
- [ ] `grep -r "math/rand" internal/codes` no devuelve nada.

---

## Errores comunes

| Síntoma | Causa | Solución |
|---|---|---|
| La prueba de "vecinos invertidos" falla en algunos casos | Es el par `09`/`90`, límite de Luhn | Excluir explícitamente ese par en la prueba, con comentario |
| `Normalize` acepta códigos con espacio inicial pero no con `\t` | Solo se recortan espacios y `\n` | `strings.TrimSpace` cubre `\t`, `\r`, `\n`; aplícalo antes del bucle |
| Pánico con string vacío en `Last4` | Índice fuera de rango | Validar longitud; `Last4` solo recibe códigos normalizados |

---

## Cómo pedírselo a Claude

```text
Usa /golang-how-to (golang-security, golang-testing, golang-safety, golang-naming).
Implementa el playbook docs/playbooks/04-generador-codigos.md en internal/codes.
Parte del código de SD §6.1. Escribe primero las pruebas con tabla del paso 2 y
luego el código hasta que pasen. Incluye el fuzz test y el ejemplo.
Después explícame, línea por línea, luhnCheckDigit y una de las pruebas con tabla.
```

> Sugerencia de aprendizaje: este es un buen playbook para que **tú** intentes escribir `Last4` y `Display` antes de pedírselo a Claude. Son 5 líneas cada una.

---

## Próximos pasos

- ➡️ **06 — CLI** usa `GenerateAPIKey` y `Hash` para crear keys.
- ➡️ **08 — Emisión** usa `Generate`, `Hash`, `Last4`, `Display`.
- ➡️ **09 — Canje** usa `Normalize` y `MaskEmail`.
- Si algún día se pide un código corto (SD §10, `ALNUM_16`), se agrega aquí una función nueva y nada más.
