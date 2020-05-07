package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bhmj/pg-api/internal/app/app"
	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/env"
	"github.com/bhmj/pg-api/internal/pkg/log"
)

const (
	envConfigPath = "PG_API_CONFIG"
)

const appVersion string = "0.3.0"

func main() {

	flag.Parse()

	cpath := env.GetString(envConfigPath, flag.Arg(0))

	fmt.Printf("PostgreSQL web API service ver. %s\n", appVersion)
	fmt.Printf("Current directory is %s\n", env.GetCurrentDir())
	fmt.Printf("Config path is %s\n", cpath)

	cfg, err := config.Read(cpath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	logger, err := log.New(cfg.LogLevel)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer logger.L().Sync()

	app.New(cfg, logger).Run()
}
