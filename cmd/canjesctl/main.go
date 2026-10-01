// Command canjesctl administra canjes-api: migraciones, apps cliente, API keys,
// campañas, ofertas y semillas. Los subcomandos se implementan en el playbook 06.
package main

import (
	"fmt"
	"io"
	"os"
)

// version se reemplaza al compilar con -ldflags "-X main.version=<versión>".
var version = "dev"

const usage = `canjesctl %s — administración de canjes-api

Uso:
  canjesctl <comando> [opciones]

Comandos (disponibles a partir del playbook 06):
  migrate    up | down | status
  apps       create | list
  keys       create | list | revoke
  campaigns  create | activate | close
  offers     create | list
  seed       <archivo.sql>
  version    muestra la versión
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run devuelve el código de salida del proceso; separarlo de main permite probarlo.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(stdout, usage, version)
		return 0
	}
	if args[0] == "version" {
		fmt.Fprintf(stdout, "canjesctl %s\n", version)
		return 0
	}
	fmt.Fprintf(stderr, "canjesctl: comando desconocido o aún no implementado: %q\n\n", args[0])
	fmt.Fprintf(stderr, usage, version)
	return 2
}
