package http

import (
	"net/http"

	"github.com/bhmj/pg-api/internal/pkg/config"
)

// HeaderValue defines HTTP header passed into fn
type HeaderValue struct {
	Name  string
	Value string
	Type  string
}

// ExtractHeaders returns a slice of structs filled with target field names
// and values from HTTP headers along with optional argument types (int or string)
func ExtractHeaders(headersToPass []config.HeaderPass, headers http.Header) []HeaderValue {
	result := make([]HeaderValue, len(headersToPass))
	for i := range headersToPass {
		canonicalHeaderKey := http.CanonicalHeaderKey(headersToPass[i].Header)
		val, found := headers[canonicalHeaderKey]
		value := ""
		if found {
			value = val[0]
		}
		result[i].Name = headersToPass[i].FieldName
		result[i].Type = headersToPass[i].ArgumentType
		result[i].Value = value
	}
	return result
}
