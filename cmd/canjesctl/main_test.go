package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "sin argumentos muestra ayuda", args: nil, wantCode: 0, wantStdout: "Uso:"},
		{name: "help muestra ayuda", args: []string{"help"}, wantCode: 0, wantStdout: "Uso:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "canjesctl dev"},
		{name: "comando desconocido", args: []string{"borrar-todo"}, wantCode: 2, wantStderr: "comando desconocido"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("código de salida = %d, se esperaba %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, debía contener %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, debía contener %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
