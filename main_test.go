package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=secretpassword dbname=department_management_test port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Очистка таблиц
	db.Exec("TRUNCATE TABLE employees, departments RESTART IDENTITY CASCADE")
	return db
}

func TestCreateDepartment_UniqueConstraint(t *testing.T) {
	db := setupTestDB(t)
	app := &App{DB: db, Logger: nil} // логгер можно nil, т.к. в тестах не критично

	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", app.handleCreateDepartment)

	// 1. Успешное создание
	body1, _ := json.Marshal(CreateDeptInput{Name: "  Backend  "})
	req1 := httptest.NewRequest("POST", "/departments/", bytes.NewBuffer(body1))
	rr1 := httptest.NewRecorder()
	mux.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusCreated, rr1.Code)

	var dept1 Department
	err := json.Unmarshal(rr1.Body.Bytes(), &dept1)
	require.NoError(t, err)
	assert.Equal(t, "Backend", dept1.Name)

	// 2. Дубликат – должно вернуть 400
	req2 := httptest.NewRequest("POST", "/departments/", bytes.NewBuffer(body1))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusBadRequest, rr2.Code)

	var errResp map[string]string
	json.Unmarshal(rr2.Body.Bytes(), &errResp)
	assert.Contains(t, errResp["error"], "already exists")
}

func TestCreateEmployee_InvalidDepartment(t *testing.T) {
	db := setupTestDB(t)
	app := &App{DB: db, Logger: nil}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/{id}/employees/", app.handleCreateEmployee)

	body, _ := json.Marshal(CreateEmpInput{FullName: "John", Position: "Dev"})
	req := httptest.NewRequest("POST", "/departments/999/employees/", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateDepartment_CycleDetection(t *testing.T) {
	db := setupTestDB(t)
	app := &App{DB: db, Logger: nil}

	// Создаём два департамента: A (id=1), B (id=2)
	deptA := Department{Name: "A", ParentID: nil}
	deptB := Department{Name: "B", ParentID: nil}
	db.Create(&deptA)
	db.Create(&deptB)

	// Пытаемся переместить A внутрь B (это разрешено)
	// А затем B внутрь A – должно быть запрещено (цикл)
	// Сначала делаем A дочерним для B
	parentPtr := uint(deptB.ID)
	updateBody, _ := json.Marshal(UpdateDeptInput{ParentID: &parentPtr})
	req := httptest.NewRequest("PATCH", "/departments/"+uintToString(deptA.ID), bytes.NewBuffer(updateBody))
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /departments/{id}", app.handleUpdateDepartment)
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Теперь пытаемся B сделать дочерним для A (цикл)
	parentPtr2 := uint(deptA.ID)
	updateBody2, _ := json.Marshal(UpdateDeptInput{ParentID: &parentPtr2})
	req2 := httptest.NewRequest("PATCH", "/departments/"+uintToString(deptB.ID), bytes.NewBuffer(updateBody2))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestDeleteDepartment_Cascade(t *testing.T) {
	db := setupTestDB(t)
	app := &App{DB: db, Logger: nil}

	// Создаём департамент, дочерний департамент и сотрудника
	parent := Department{Name: "Parent"}
	child := Department{Name: "Child", ParentID: &parent.ID}
	db.Create(&parent)
	child.ParentID = &parent.ID
	db.Create(&child)
	emp := Employee{DepartmentID: child.ID, FullName: "Emp", Position: "Pos"}
	db.Create(&emp)

	// Удаляем родителя в режиме cascade
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /departments/{id}", app.handleDeleteDepartment)
	req := httptest.NewRequest("DELETE", "/departments/"+uintToString(parent.ID)+"?mode=cascade", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Проверяем, что всё удалилось
	var count int64
	db.Model(&Department{}).Count(&count)
	assert.Equal(t, int64(0), count)
	db.Model(&Employee{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// Вспомогательная функция
func uintToString(u uint) string {
	return strconv.FormatUint(uint64(u), 10)
}

// Добавьте import "strconv" в начало файла
