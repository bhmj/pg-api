package http

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server implements basic Kube-dispatched HTTP server
type Server interface {
	Run(cfg config.HTTP, log log.Logger)
	Shutdown()
	HandleFunc(pattern string, handler http.HandlerFunc)
	//
	Ready()
	NotReady()
}

type server struct {
	mx    sync.RWMutex
	wg    sync.WaitGroup
	ready bool
	alive bool
	mux   *http.ServeMux
	srv   *http.Server
	log   log.Logger
}

// NewServer returns an HTTP server
func NewServer() Server {
	mux := http.NewServeMux()
	srv := &server{
		wg:  sync.WaitGroup{},
		mux: mux,
		srv: &http.Server{
			ReadHeaderTimeout: 1 * time.Second,
			Handler:           mux,
		},
	}

	// request multiplexing
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) })
	mux.HandleFunc("/ready", srv.handleIsReady)
	mux.HandleFunc("/alive", srv.handleIsAlive)

	return srv
}

// Run the server
func (s *server) Run(cfg config.HTTP, log log.Logger) {
	// set server port
	s.srv.Addr = fmt.Sprintf(":%d", cfg.Port) // TODO: remove?
	s.log = log
	// reuse port
	listener, err := Listener(cfg.Port)
	if err != nil {
		s.log.L().Fatal("failed to create listener: ", err)
	}

	go func() {
		if cfg.UseSSL {
			s.srv.ServeTLS(listener, cfg.SSLCert, cfg.SSLKey)
		} else {
			s.srv.Serve(listener)
		}
	}()

	println("listening at ", cfg.Port)
}

// Shutdown the server
func (s *server) Shutdown() {
	s.mx.Lock()
	s.alive = false // k8s liveness probe
	s.ready = false // k8s readiness probe
	s.mx.Unlock()

	s.srv.SetKeepAlivesEnabled(false)

	// 5 sec timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer func() {
		cancel()
	}()
	if err := s.srv.Shutdown(ctx); err != nil {
		s.log.L().Fatal("server shutdown failed: ", err)
	}
	s.log.L().Info("server shutdown ok")
}

func (s *server) IsReady() bool {
	s.mx.RLock()
	r := s.ready
	s.mx.RUnlock()
	return r
}

func (s *server) Ready() {
	s.mx.Lock()
	s.ready = true
	s.mx.Unlock()
}

func (s *server) NotReady() {
	s.mx.Lock()
	s.ready = false
	s.mx.Unlock()
}

func (s *server) IsAlive() bool {
	s.mx.RLock()
	r := s.alive
	s.mx.RUnlock()
	return r
}

func (s *server) Animate() {
	s.mx.Lock()
	s.alive = true
	s.mx.Unlock()
}

func (s *server) handleIsReady(w http.ResponseWriter, r *http.Request) {
	if s.IsReady() {
		w.WriteHeader(http.StatusOK)
	} else {
		http.NotFound(w, r)
	}
}

func (s *server) handleIsAlive(w http.ResponseWriter, r *http.Request) {
	if s.IsAlive() {
		w.WriteHeader(http.StatusOK)
	} else {
		http.NotFound(w, r)
	}
}

// Handle used as a normal mux.Handler with additional wg counter
func (s *server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, s.waitable(handler))
}

func (s *server) waitable(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.wg.Add(1)
		handler(w, r)
		s.wg.Done()
	}
}
