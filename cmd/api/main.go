package main

import (
	"database/sql"
	"dept-api/internal/config"
	"dept-api/internal/handler"
	"dept-api/internal/middleware"
	"dept-api/internal/repository"
	"dept-api/internal/service"
	"log/slog"
	"net/http"
	"os"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Подключение для миграций
	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open sql db", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("goose dialect error", "error", err)
		os.Exit(1)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// GORM
	gormDB, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Error("failed to connect gorm", "error", err)
		os.Exit(1)
	}

	// Инициализация репозиториев
	deptRepo := repository.NewDepartmentRepository(gormDB)
	empRepo := repository.NewEmployeeRepository(gormDB)

	// Инициализация сервисов
	deptService := service.NewDepartmentService(gormDB, deptRepo, empRepo)
	empService := service.NewEmployeeService(empRepo, deptRepo)

	// Хендлеры
	deptHandler := handler.NewDepartmentHandler(deptService, empService)

	// Роутер
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", deptHandler.CreateDepartment)
	mux.HandleFunc("POST /departments/{id}/employees/", deptHandler.CreateEmployee)
	mux.HandleFunc("GET /departments/{id}", deptHandler.GetDepartment)
	mux.HandleFunc("PATCH /departments/{id}", deptHandler.UpdateDepartment)
	mux.HandleFunc("DELETE /departments/{id}", deptHandler.DeleteDepartment)

	// Middleware
	handlerWithLogging := middleware.LoggingMiddleware(logger)(mux)

	logger.Info("starting server", slog.String("port", cfg.Port))
	if err := http.ListenAndServe(cfg.Port, handlerWithLogging); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
