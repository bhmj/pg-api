package service

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/bhmj/pg-api/internal/app/handle"
	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/log"
)

// Service is an interface
type Service interface {
	Run() error
}

type service struct {
	cfg *config.Config
	log log.Logger
}

// New creates service
func New(cfg *config.Config, log log.Logger) Service {
	return &service{cfg: cfg, log: log}
}

func (s *service) Run() error {
	var wg sync.WaitGroup
	handler := handle.Root(s.cfg, s.log, &wg)
	http.HandleFunc("/", handler) // each request calls handler
	err := http.ListenAndServe("localhost:"+strconv.Itoa(s.cfg.HTTP.Port), nil)
	return err
}
