package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// BootstrapConfig holds the minimal config needed before database connection.
type BootstrapConfig struct {
	DatabaseURL string
	RedisURL    string // optional override; empty means use DB setting
	Listen      string
	JFListen    string
	Mode        string
}

// LoadBootstrap loads bootstrap configuration from a .env file (if it exists)
// and environment variables. Only DATABASE_URL is required.
func LoadBootstrap(envFile string) (*BootstrapConfig, error) {
	if envFile != "" {
		_ = godotenv.Load(envFile)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (set in .env or environment)")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jfPort := os.Getenv("JF_PORT")
	if jfPort == "" {
		jfPort = "8096"
	}

	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "integrated"
	}

	redisURL := os.Getenv("REDIS_URL")

	return &BootstrapConfig{
		DatabaseURL: dbURL,
		RedisURL:    redisURL,
		Listen:      ":" + port,
		JFListen:    ":" + jfPort,
		Mode:        mode,
	}, nil
}
