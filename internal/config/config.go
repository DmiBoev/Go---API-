package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
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
	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
	}
}
