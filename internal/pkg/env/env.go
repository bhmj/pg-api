package env

import (
	"os"
	"path/filepath"
)

// GetCurrentDir does what the name says
func GetCurrentDir() string {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "?/"
	}
	return currentDir
}

// GetString returns env value of default
func GetString(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
