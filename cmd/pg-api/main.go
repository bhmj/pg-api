package main

import (
	"fmt"

	"github.com/bhmj/pg-api/internal/env"
)

const appVersion string = "0.1.0"

func main() {
	fmt.Printf("PostgreSQL web API service ver. %s\n", appVersion)
	fmt.Printf("Current directory is %s\n", env.GetCurrentDir())
}
