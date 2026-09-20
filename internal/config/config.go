package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr string
}

func Load() (Config, error) {
	addr := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	if !strings.Contains(addr, ":") {
		return Config{}, fmt.Errorf("HTTP_ADDR must contain a port: %q", addr)
	}
	return Config{HTTPAddr: addr}, nil
}
