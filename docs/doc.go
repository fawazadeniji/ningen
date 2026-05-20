// Package docs exposes the OpenAPI specification as an embedded byte slice.
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
