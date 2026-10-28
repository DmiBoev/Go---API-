//go:build integration

package service

import (
	"context"
	"dept-api/internal/domain"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"testing"

	"dept-api/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupServiceTestDB(t *testing.T) *sqlx.DB { return testutil.NewDB(t) }

func TestDepartmentService_CreateDuplicateName(t *testing.T) {
	db := setupServiceTestDB(t)
	deptRepo := repository.NewDepartmentRepository(db)

	svc := NewDepartmentService(deptRepo)
	ctx := context.Background()

	input := models.CreateDepartmentInput{Name: "HR"}
	_, err := svc.Create(ctx, input)
	require.NoError(t, err)

	_, err = svc.Create(ctx, input)
	assert.Error(t, err) // дубликат
}

func TestDepartmentService_UpdateCycle(t *testing.T) {
	db := setupServiceTestDB(t)
	deptRepo := repository.NewDepartmentRepository(db)

	svc := NewDepartmentService(deptRepo)
	ctx := context.Background()

	// Создаём A и B
	a, _ := svc.Create(ctx, models.CreateDepartmentInput{Name: "A"})
	b, _ := svc.Create(ctx, models.CreateDepartmentInput{Name: "B"})

	// Перемещаем A в B (разрешено)
	newParent := b.ID
	_, err := svc.Update(ctx, a.ID, models.UpdateDepartmentInput{ParentID: &newParent})
	require.NoError(t, err)

	// Пытаемся переместить B в A (цикл)
	_, err = svc.Update(ctx, b.ID, models.UpdateDepartmentInput{ParentID: &a.ID})
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestDepartmentService_DeleteReassign(t *testing.T) {
	db := setupServiceTestDB(t)
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	svc := NewDepartmentService(deptRepo)
	ctx := context.Background()

	parent, _ := svc.Create(ctx, models.CreateDepartmentInput{Name: "Parent"})
	child, _ := svc.Create(ctx, models.CreateDepartmentInput{Name: "Child", ParentID: &parent.ID})
	target, _ := svc.Create(ctx, models.CreateDepartmentInput{Name: "Target"})

	emp := &models.Employee{DepartmentID: child.ID, FullName: "Emp", Position: "Pos"}
	require.NoError(t, empRepo.Create(ctx, emp))

	// Удаляем child с переназначением сотрудников в target
	err := svc.Delete(ctx, child.ID, "reassign", &target.ID)
	require.NoError(t, err)

	// Сотрудник должен быть в target
	empAfter, _ := empRepo.GetByID(ctx, emp.ID)
	assert.Equal(t, target.ID, empAfter.DepartmentID)

	// Дочерние подразделения должны переподчиниться родителю удалённого
	// (в данном случае у child не было своих детей, но если бы были – их parent_id стал бы parent.ID)
}
