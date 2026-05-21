package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"://github.com"
	"://github.com"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL env not set, skipping integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Очищаем таблицы перед тестом
	db.Exec("TRUNCATE TABLE employees, departments RESTART IDENTITY CASCADE")
	return db
}

func TestCreateDepartmentUniqueValidation(t *testing.T) {
	db := setupTestDB(t)
	app := &App{DB: db, Logger: http.NoBody.(interface{}).(*gorm.DB /*mock или заглушка для логгера*/)} // Можно использовать дефолтный логгер
	
	// Используем встроенный роутер
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", app.handleCreateDepartment)

	// 1. Создаем первый департамент
	body, _ := json.Marshal(CreateDeptInput{Name: "  Backend  "})
	req := httptest.NewRequest("POST", "/departments/", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var created Department
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	assert.Equal(t, "Backend", created.Name) // Тримминг пробелов

	// 2. Пробуем создать дубликат на том же корневом уровне
	req2 := httptest.NewRequest("POST", "/departments/", bytes.NewBuffer(body))
	rr2 := httptest.NewRecorder()
	
	mux.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusBadRequest, rr2.Code) // Ошибка уникальности
}
