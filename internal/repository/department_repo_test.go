//go:build integration

package repository

import (
	"context"
	"dept-api/internal/models"
	"testing"

	"dept-api/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepoTestDB(t *testing.T) *sqlx.DB { return testutil.NewDB(t) }

func TestDepartmentRepository_CreateAndGet(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewDepartmentRepository(db)

	ctx := context.Background()
	dept := &models.Department{Name: "Sales"}
	err := repo.Create(ctx, dept)
	require.NoError(t, err)
	assert.NotZero(t, dept.ID)

	fetched, err := repo.GetByID(ctx, dept.ID)
	require.NoError(t, err)
	assert.Equal(t, "Sales", fetched.Name)
}

func TestDepartmentRepository_UniqueConstraint(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewDepartmentRepository(db)
	ctx := context.Background()

	parent := &models.Department{Name: "Parent"}
	require.NoError(t, repo.Create(ctx, parent))
	parentID := parent.ID
	dept1 := &models.Department{Name: "IT", ParentID: &parentID}
	err := repo.Create(ctx, dept1)
	require.NoError(t, err)

	dept2 := &models.Department{Name: "IT", ParentID: &parentID}
	err = repo.Create(ctx, dept2)
	assert.Error(t, err) // duplicate key violation
}

func TestDepartmentRepository_CheckCyclicMove(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewDepartmentRepository(db)
	ctx := context.Background()

	// A -> B -> C
	a := &models.Department{Name: "A"}
	b := &models.Department{Name: "B", ParentID: &a.ID}
	c := &models.Department{Name: "C", ParentID: &b.ID}
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.Create(ctx, b))
	require.NoError(t, repo.Create(ctx, c))

	// Проверка: можно ли переместить A внутрь C? (цикл)
	cyclic, err := repo.CheckCyclicMove(ctx, a.ID, c.ID)
	require.NoError(t, err)
	assert.True(t, cyclic)

	// Перемещение C внутрь A (допустимо)
	cyclic, err = repo.CheckCyclicMove(ctx, c.ID, a.ID)
	require.NoError(t, err)
	assert.False(t, cyclic)
}

func TestDepartmentRepository_ReassignEmployees(t *testing.T) {
	db := setupRepoTestDB(t)
	deptRepo := NewDepartmentRepository(db)
	empRepo := NewEmployeeRepository(db)
	ctx := context.Background()

	fromDept := &models.Department{Name: "Old"}
	toDept := &models.Department{Name: "New"}
	require.NoError(t, deptRepo.Create(ctx, fromDept))
	require.NoError(t, deptRepo.Create(ctx, toDept))

	emp := &models.Employee{DepartmentID: fromDept.ID, FullName: "John", Position: "Dev"}
	require.NoError(t, empRepo.Create(ctx, emp))

	err := deptRepo.ReassignEmployees(ctx, fromDept.ID, toDept.ID)
	require.NoError(t, err)

	empAfter, _ := empRepo.GetByID(ctx, emp.ID)
	assert.Equal(t, toDept.ID, empAfter.DepartmentID)
}
