// Command canjes-api es el servidor HTTP de emisión y canje de códigos.
//
// Por ahora solo imprime su versión; el servidor se arma en el playbook 05.
package main

import (
	"fmt"
	"os"
)

// version se reemplaza al compilar con -ldflags "-X main.version=<versión>".
var version = "dev"

func main() {
	if _, err := fmt.Fprintf(os.Stdout, "canjes-api %s\n", version); err != nil {
		os.Exit(1)
	}
}
