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
