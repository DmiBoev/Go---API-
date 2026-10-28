package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	Port               string
	CORSAllowedOrigins []string
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "host=localhost user=postgres password=secretpassword dbname=department_management port=5432 sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if origins == "" {
		origins = "*"
	}
	return &Config{
		DatabaseURL:        dbURL,
		Port:               port,
		CORSAllowedOrigins: strings.Split(origins, ","),
	}
}
