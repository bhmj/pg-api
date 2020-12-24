package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bhmj/pg-api/internal/app/service"
	"github.com/bhmj/pg-api/internal/pkg/auth"
	"github.com/bhmj/pg-api/internal/pkg/config"
	phttp "github.com/bhmj/pg-api/internal/pkg/http"
	"github.com/bhmj/pg-api/internal/pkg/log"
)

// App ...
type App interface {
	Run()
}

type app struct {
	cfg *config.Config
	log log.Logger
}

// New creates service
func New(cfg *config.Config, log log.Logger) App {
	return &app{cfg: cfg, log: log}
}

// Run runs service
func (s *app) Run() {
	var v *auth.Verifier
	var err error
	//
	srv := phttp.NewServer()
	// key access auth
	if len(s.cfg.HTTP.AccessFiles) > 0 {
		v, err = auth.NewVerifier(s.log, s.cfg.HTTP.AccessFiles)
		if err != nil {
			s.log.L().Error(err)
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	// add handlers
	svc, err := service.NewService(ctx, s.cfg, s.log, srv)
	if err != nil {
		cancel()
		s.log.L().Error(err)
		return
	}
	mainHandler := svc.MainHandler
	fileHandler := svc.FileHandler
	if v != nil {
		mainHandler = v.Wrap(mainHandler)
		fileHandler = v.Wrap(fileHandler)
	}
	srv.HandleFunc("/"+s.cfg.HTTP.Endpoint+"/", mainHandler)
	srv.HandleFunc("/"+s.cfg.HTTP.Endpoint+"/file/", fileHandler)
	// run HTTP server
	srv.Run(s.cfg.HTTP, s.log)
	// signal processing
	done := make(chan bool)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		err := fmt.Errorf("%s", <-c)
		s.log.L().Info("signal: ", err)
		tim := time.Now()
		cancel()       // query termination
		srv.Shutdown() // server termination
		s.log.L().Info("shutdown duration: ", time.Since(tim))
		done <- true
	}()
	<-done
}
