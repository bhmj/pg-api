package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/bhmj/pg-api/internal/app/service"
	"github.com/bhmj/pg-api/internal/pkg/env"
	"github.com/bhmj/pg-api/internal/pkg/config"
)

const appVersion string = "0.1.0"

func main() {
	
	var logger log.Logger
	
	fmt.Printf("PostgreSQL web API service ver. %s\n", appVersion)
	fmt.Printf("Current directory is %s\n", env.GetCurrentDir())

	cfg, err := config.Read(getEnv(envConfigPath, flag.Arg(0))
	if err != nil {
		logger.Log("msg", "failed to load config", "err", err)
		os.Exit(1)
	}

	srv := service.New(cfg)
	srv.Run()

	http.HandleFunc("/", handler) // each request calls handler
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "URL.Path = %q\n", r.URL.Path)
}
