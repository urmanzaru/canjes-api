package httpapi_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/urmanzaru/canjes-api/internal/httpapi/gen"
)

// allowedScopes son los únicos permisos que puede tener una API key (SD §0.1).
var allowedScopes = []string{"offers:read", "codes:issue", "codes:redeem", "admin"}

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()

	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("no se pudo cargar el contrato incrustado: %v", err)
	}
	return spec
}

func TestContract_IsValidOpenAPI(t *testing.T) {
	t.Parallel()

	spec := loadSpec(t)
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml no es un OpenAPI válido: %v", err)
	}
}

// TestContract_OperationRules verifica las convenciones de SD §5 en cada operación.
func TestContract_OperationRules(t *testing.T) {
	t.Parallel()

	spec := loadSpec(t)
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			t.Run(method+" "+path, func(t *testing.T) {
				t.Parallel()

				scope, hasScope := op.Extensions["x-required-scope"].(string)
				isV1 := strings.HasPrefix(path, "/v1/")

				switch {
				case isV1 && !hasScope:
					t.Errorf("falta x-required-scope")
				case isV1 && !slices.Contains(allowedScopes, scope):
					t.Errorf("x-required-scope %q no es uno de %v", scope, allowedScopes)
				case !isV1 && (op.Security == nil || len(*op.Security) != 0):
					t.Errorf("las rutas fuera de /v1 deben declarar security: [] (sin autenticación)")
				}

				if isV1 {
					for _, code := range []string{"401", "403", "500"} {
						if op.Responses.Value(code) == nil {
							t.Errorf("falta la respuesta %s", code)
						}
					}
				}

				for code, ref := range op.Responses.Map() {
					status, err := strconv.Atoi(code)
					if err != nil || status < 400 {
						continue
					}
					media := ref.Value.Content.Get("application/json")
					if media == nil || media.Schema == nil || !strings.HasSuffix(media.Schema.Ref, "/ApiError") {
						t.Errorf("la respuesta %s debe usar el esquema ApiError (D13)", code)
					}
				}
			})
		}
	}
}
