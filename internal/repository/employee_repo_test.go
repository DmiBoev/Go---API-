//go:build integration

package repository

import (
	"context"
	"dept-api/internal/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmployeeRepositoryCRUD(t *testing.T) {
	db := setupRepoTestDB(t)
	departments := NewDepartmentRepository(db)
	employees := NewEmployeeRepository(db)
	ctx := context.Background()
	dept := &models.Department{Name: "Engineering"}
	require.NoError(t, departments.Create(ctx, dept))
	emp := &models.Employee{DepartmentID: dept.ID, FullName: "Test Employee", Position: "Developer"}
	require.NoError(t, employees.Create(ctx, emp))
	require.NotZero(t, emp.ID)
	assert.False(t, emp.CreatedAt.IsZero())
	fetched, err := employees.GetByID(ctx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Nil(t, fetched.HiredAt)
	fetched.Position = "Lead"
	require.NoError(t, employees.Update(ctx, fetched))
	updated, err := employees.GetByID(ctx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Lead", updated.Position)
	require.NoError(t, employees.Delete(ctx, emp.ID))
	missing, err := employees.GetByID(ctx, emp.ID)
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestRepositoryCancelledContext(t *testing.T) {
	db := setupRepoTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewDepartmentRepository(db).GetByID(ctx, 1)
	assert.ErrorIs(t, err, context.Canceled)
}
