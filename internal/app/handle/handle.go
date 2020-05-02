package handle

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/log"
)

// Root defines root handler
func Root(cfg *config.Config, log log.Logger, wg *sync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		fmt.Fprintf(w, "URL.Path = %q\n", r.URL.Path)
		wg.Done()
	}
}
