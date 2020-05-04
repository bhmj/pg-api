package auth

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bhmj/pg-api/internal/pkg/log"
)

const (
	unknownCaller = "unknown"
	// KeyHeader contains secret private key of a remote service
	KeyHeader = "X-Auth-Sign"
	// CallerHeader contains name of a remote service
	CallerHeader = "X-Auth-Id"
)

// Verifier parses and verifies access keys.
type Verifier struct {
	logger      log.Logger
	accessFiles []string
	accessKeys  map[string]string
}

// NewVerifier creates a new Verifier.
func NewVerifier(logger log.Logger, accessFiles []string) (*Verifier, error) {
	if len(accessFiles) == 0 {
		return nil, fmt.Errorf("must provide AccessFiles")
	}
	if logger == nil {
		return nil, fmt.Errorf("must provide logger")
	}

	v := &Verifier{
		accessFiles: accessFiles,
		accessKeys:  make(map[string]string),
		logger:      logger,
	}

	return v, v.loadKeysFromFiles()
}

func (v *Verifier) loadKeysFromFiles() error {
	for _, f := range v.accessFiles {
		keysBlock, err := os.Open(f)
		if err != nil {
			return fmt.Errorf("failed to open file: %v", err)
		}
		defer keysBlock.Close()

		err = v.loadKeys(keysBlock)
		if err != nil {
			return fmt.Errorf("failed to loads keys: %v", err)
		}
	}
	return nil
}

// loadKeys load access keys from the given io.Reader and
// represents them as map[string]string where key is caller, value is access key
func (v *Verifier) loadKeys(keysBlock io.Reader) error {
	lines, err := csv.NewReader(keysBlock).ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read records: %v", err)
	}

	for i, line := range lines {
		if len(line) != 2 {
			return fmt.Errorf("invalid format: line %d", i)
		}

		key := strings.TrimSpace(line[0])
		caller := strings.TrimSpace(line[1])
		if key == "" || caller == "" {
			return fmt.Errorf("invalid key or caller")
		}

		v.accessKeys[caller] = key
	}

	return nil
}

// Wrap wraps an HTTP handler with a middleware that acts as a access limiter.
// Requests which wouldn't have an access key are simply rejected with an error.
// The successfully verified caller is saved to the request's context.
func (v *Verifier) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := r.Header.Get(CallerHeader)
		key, ok := v.accessKeys[caller]
		if !ok || key != r.Header.Get(KeyHeader) {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Access to production service\"")
			v.logger.L().Errorf("access key verification error for caller %s", caller)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		r = r.WithContext(SetCaller(r.Context(), caller))
		next.ServeHTTP(w, r)
	})
}

// GetAccessKey returns a key associated with the given caller or ''.
func (v *Verifier) GetAccessKey(caller string) string {
	return v.accessKeys[caller]
}

// callerContextKey is the type to use with context's WithValue
// function to associate an caller value with a context.
type callerContextKey struct{}

// SetCaller returns a copy of context associated with the given caller.
func SetCaller(ctx context.Context, caller string) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller)
}

// GetCaller returns a caller associated with the given context or 'unknown'.
func GetCaller(ctx context.Context) string {
	caller, ok := ctx.Value(callerContextKey{}).(string)
	if !ok || caller == "" {
		return unknownCaller
	}
	return caller
}
