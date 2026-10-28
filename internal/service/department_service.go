package service

import (
	"context"
	"dept-api/internal/domain"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"fmt"
	"strings"
	"unicode/utf8"
)

type DepartmentService interface {
	Create(context.Context, models.CreateDepartmentInput) (*models.Department, error)
	GetByID(context.Context, uint, int, bool) (*models.Department, error)
	Update(context.Context, uint, models.UpdateDepartmentInput) (*models.Department, error)
	Delete(context.Context, uint, string, *uint) error
}

type departmentService struct {
	deptRepo repository.DepartmentRepository
}

func NewDepartmentService(deptRepo repository.DepartmentRepository) DepartmentService {
	return &departmentService{deptRepo: deptRepo}
}

func validateString(field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > maxLen {
		return "", fmt.Errorf("%w: length must be between 1 and %d characters", domain.ErrInvalid, maxLen)
	}
	return trimmed, nil
}

func findDepartment(ctx context.Context, repo repository.DepartmentRepository, id uint) (*models.Department, error) {
	dept, err := repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if dept == nil {
		return nil, fmt.Errorf("%w: department not found", domain.ErrNotFound)
	}
	return dept, nil
}

func (s *departmentService) Create(ctx context.Context, input models.CreateDepartmentInput) (*models.Department, error) {
	name, err := validateString(input.Name, 200)
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil {
		if _, err := findDepartment(ctx, s.deptRepo, *input.ParentID); err != nil {
			return nil, err
		}
	}
	dept := &models.Department{Name: name, ParentID: input.ParentID}
	if err := s.deptRepo.Create(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) GetByID(ctx context.Context, id uint, depth int, includeEmployees bool) (*models.Department, error) {
	depth = max(1, min(depth, 5))
	tree, err := s.deptRepo.GetChildrenRecursive(ctx, id, depth, includeEmployees)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, fmt.Errorf("%w: department not found", domain.ErrNotFound)
	}
	return tree, nil
}

func (s *departmentService) Update(ctx context.Context, id uint, input models.UpdateDepartmentInput) (*models.Department, error) {
	var dept *models.Department
	err := s.deptRepo.WithinTransaction(ctx, func(repo repository.DepartmentRepository) error {
		var err error
		dept, err = findDepartment(ctx, repo, id)
		if err != nil {
			return err
		}
		if input.Name != nil {
			dept.Name, err = validateString(*input.Name, 200)
			if err != nil {
				return err
			}
		}
		if input.ParentID != nil {
			if *input.ParentID == id {
				return fmt.Errorf("%w: department cannot be its own parent", domain.ErrInvalid)
			}
			if _, err := findDepartment(ctx, repo, *input.ParentID); err != nil {
				return err
			}
			cyclic, err := repo.CheckCyclicMove(ctx, id, *input.ParentID)
			if err != nil {
				return err
			}
			if cyclic {
				return fmt.Errorf("%w: cannot move department inside its own descendant", domain.ErrConflict)
			}
			dept.ParentID = input.ParentID
		}
		return repo.Update(ctx, dept)
	})
	if err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) Delete(ctx context.Context, id uint, mode string, reassignTo *uint) error {
	if mode != "" && mode != "cascade" && mode != "reassign" {
		return fmt.Errorf("%w: mode must be cascade or reassign", domain.ErrInvalid)
	}
	if mode == "reassign" && (reassignTo == nil || *reassignTo == id) {
		return fmt.Errorf("%w: a different reassign_to_department_id is required", domain.ErrInvalid)
	}
	return s.deptRepo.WithinTransaction(ctx, func(repo repository.DepartmentRepository) error {
		dept, err := findDepartment(ctx, repo, id)
		if err != nil {
			return err
		}
		if mode == "reassign" {
			if _, err := findDepartment(ctx, repo, *reassignTo); err != nil {
				return err
			}
			if err := repo.ReassignEmployees(ctx, id, *reassignTo); err != nil {
				return err
			}
			if err := repo.ReassignChildrenParent(ctx, &id, dept.ParentID); err != nil {
				return err
			}
		}
		return repo.Delete(ctx, id)
	})
}
