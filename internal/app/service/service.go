package service

import (
	"github.com/bhmj/pg-api/internal/pkg/config"
)

// Service is an interface
type Service interface {
	Run()
}

type service struct {
	cfg *config.Config
}

// New creates service
func New(cfg *config.Config) Service {
	return &service{cfg: cfg}
}

func (s *service) Run() {
	return
}
