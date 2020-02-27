package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bhmj/pg-api/internal/app/service"
	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/env"
	//"github.com/go-kit/kit/log"
)

const (
	envConfigPath = "PG_API_CONFIG"
)

const appVersion string = "0.1.1"

func main() {

	//var logger log.Logger
	flag.Parse()

	fmt.Printf("PostgreSQL web API service ver. %s\n", appVersion)
	fmt.Printf("Current directory is %s\n", env.GetCurrentDir())

	cfg, err := config.Read(env.GetString(envConfigPath, flag.Arg(0)))
	if err != nil {
		fmt.Println(err.Error())
		//logger.Log("msg", "failed to load config", "err", err)
		os.Exit(1)
	}

	srv := service.New(cfg)
	fmt.Println(srv.Run())
}
