//go:build integration

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

	"dept-api/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sqlx.DB { return testutil.NewDB(t) }

func TestCreateDepartment(t *testing.T) {
	db := setupTestDB(t)
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	deptService := service.NewDepartmentService(deptRepo)
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
