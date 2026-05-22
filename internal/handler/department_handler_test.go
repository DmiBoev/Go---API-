package handler

import (
	"bytes"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"dept-api/internal/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Используем строку подключения (лучше через переменную окружения)
	dsn := "host=localhost user=postgres password=secretpassword dbname=department_management_test port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Очистка таблиц
	db.Exec("TRUNCATE TABLE employees, departments RESTART IDENTITY CASCADE")
	return db
}

func TestCreateDepartment(t *testing.T) {
	db := setupTestDB(t)
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	deptService := service.NewDepartmentService(db, deptRepo, empRepo)
	empService := service.NewEmployeeService(empRepo, deptRepo)
	handler := NewDepartmentHandler(deptService, empService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", handler.CreateDepartment)

	body, _ := json.Marshal(models.CreateDepartmentInput{Name: "Test Dept"})
	req := httptest.NewRequest("POST", "/departments/", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var dept models.Department
	err := json.Unmarshal(rr.Body.Bytes(), &dept)
	require.NoError(t, err)
	assert.Equal(t, "Test Dept", dept.Name)
}
