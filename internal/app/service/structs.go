package service

import (
	"github.com/bhmj/pg-api/internal/pkg/config"
)

// ParsedURL contains parsed data from query URL
type ParsedURL struct {
	MethodPath string // "/path/to/method/"
	QueryPath  string // "path_to_method"
	ID         []int64
	config.MethodConfig
}
