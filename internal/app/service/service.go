package service

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bhmj/pg-api/internal/pkg/config"
)

// Service is an interface
type Service interface {
	Run() error
}

type service struct {
	cfg *config.Config
}

// New creates service
func New(cfg *config.Config) Service {
	return &service{cfg: cfg}
}

func (s *service) Run() error {
	http.HandleFunc("/", handler) // each request calls handler
	return http.ListenAndServe("localhost:"+strconv.Itoa(s.cfg.HTTP.Port), nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "URL.Path = %q\n", r.URL.Path)
}
