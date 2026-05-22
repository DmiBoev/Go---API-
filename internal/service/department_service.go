package service

import (
	"context"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type DepartmentService interface {
	Create(ctx context.Context, input models.CreateDepartmentInput) (*models.Department, error)
	GetByID(ctx context.Context, id uint, depth int, includeEmployees bool) (*models.Department, error)
	Update(ctx context.Context, id uint, input models.UpdateDepartmentInput) (*models.Department, error)
	Delete(ctx context.Context, id uint, mode string, reassignTo *uint) error
}

type departmentService struct {
	deptRepo repository.DepartmentRepository
	empRepo  repository.EmployeeRepository
	db       *gorm.DB
}

func NewDepartmentService(db *gorm.DB, deptRepo repository.DepartmentRepository, empRepo repository.EmployeeRepository) DepartmentService {
	return &departmentService{
		deptRepo: deptRepo,
		empRepo:  empRepo,
		db:       db,
	}
}

// validateString общая валидация строк
func validateString(field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" || len(trimmed) > maxLen {
		return "", fmt.Errorf("длина должна быть от 1 до %d символов", maxLen)
	}
	return trimmed, nil
}

func (s *departmentService) Create(ctx context.Context, input models.CreateDepartmentInput) (*models.Department, error) {
	name, err := validateString(input.Name, 200)
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil {
		parent, err := s.deptRepo.GetByID(ctx, *input.ParentID)
		if err != nil || parent == nil {
			return nil, errors.New("parent department not found")
		}
	}
	dept := &models.Department{
		Name:     name,
		ParentID: input.ParentID,
	}
	err = s.deptRepo.Create(ctx, dept)
	if err != nil {
		// GORM может вернуть ошибку уникальности, но здесь она будет обёрнута
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) GetByID(ctx context.Context, id uint, depth int, includeEmployees bool) (*models.Department, error) {
	if depth < 1 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}
	tree, err := s.deptRepo.GetChildrenRecursive(ctx, id, depth, includeEmployees)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, errors.New("department not found")
	}
	return tree, nil
}

func (s *departmentService) Update(ctx context.Context, id uint, input models.UpdateDepartmentInput) (*models.Department, error) {
	dept, err := s.deptRepo.GetByID(ctx, id)
	if err != nil || dept == nil {
		return nil, errors.New("department not found")
	}

	// Валидация имени
	if input.Name != nil {
		trimmed, err := validateString(*input.Name, 200)
		if err != nil {
			return nil, err
		}
		input.Name = &trimmed
		dept.Name = trimmed
	}

	// Обработка parent_id с проверками
	if input.ParentID != nil {
		if *input.ParentID == id {
			return nil, errors.New("department cannot be its own parent")
		}
		parent, err := s.deptRepo.GetByID(ctx, *input.ParentID)
		if err != nil || parent == nil {
			return nil, errors.New("new parent not found")
		}
		// Циклическая проверка
		cyclic, err := s.deptRepo.CheckCyclicMove(ctx, id, *input.ParentID)
		if err != nil {
			return nil, err
		}
		if cyclic {
			return nil, errors.New("cannot move department inside its own descendant")
		}
		dept.ParentID = input.ParentID
	}

	// Используем транзакцию для обновления (из-за уникальности индекса)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.deptRepo.Update(ctx, dept)
	})
	if err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) Delete(ctx context.Context, id uint, mode string, reassignTo *uint) error {
	dept, err := s.deptRepo.GetByID(ctx, id)
	if err != nil || dept == nil {
		return errors.New("department not found")
	}
	if mode == "reassign" {
		if reassignTo == nil {
			return errors.New("reassign_to_department_id is required")
		}
		target, err := s.deptRepo.GetByID(ctx, *reassignTo)
		if err != nil || target == nil {
			return errors.New("target department not found")
		}
		err = s.db.Transaction(func(tx *gorm.DB) error {
			// Перемещаем сотрудников
			if err := s.deptRepo.ReassignEmployees(ctx, id, *reassignTo); err != nil {
				return err
			}
			// Поднимаем дочерние подразделения на уровень родителя удаляемого
			if err := s.deptRepo.ReassignChildrenParent(ctx, &id, dept.ParentID); err != nil {
				return err
			}
			// Удаляем сам департамент
			if err := s.deptRepo.Delete(ctx, id); err != nil {
				return err
			}
			return nil
		})
		return err
	}
	// cascade
	return s.deptRepo.Delete(ctx, id)
}
