package main

import (
	"context"
	"dept-api/internal/config"
	"dept-api/internal/handler"
	"dept-api/internal/middleware"
	"dept-api/internal/repository"
	"dept-api/internal/service"
	"dept-api/migrations"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sqlx.ConnectContext(ctx, "postgres", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := migrations.Up(ctx, db.DB); err != nil {
		return err
	}
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	deptHandler := handler.NewDepartmentHandler(service.NewDepartmentService(deptRepo), service.NewEmployeeService(empRepo, deptRepo))
	mux := handler.NewRouter(deptHandler)
	server := &http.Server{
		Addr:              cfg.Port,
		Handler:           middleware.LoggingMiddleware(logger)(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("starting server", "port", cfg.Port)
	return server.ListenAndServe()
}
