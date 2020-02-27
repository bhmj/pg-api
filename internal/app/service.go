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

func (s *service) Run() {
	return
}
