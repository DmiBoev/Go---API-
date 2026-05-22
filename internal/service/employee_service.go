package service

import (
	"context"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"errors"
)

type EmployeeService interface {
	Create(ctx context.Context, deptID uint, input models.CreateEmployeeInput) (*models.Employee, error)
}

type employeeService struct {
	empRepo  repository.EmployeeRepository
	deptRepo repository.DepartmentRepository
}

func NewEmployeeService(empRepo repository.EmployeeRepository, deptRepo repository.DepartmentRepository) EmployeeService {
	return &employeeService{
		empRepo:  empRepo,
		deptRepo: deptRepo,
	}
}

func (s *employeeService) Create(ctx context.Context, deptID uint, input models.CreateEmployeeInput) (*models.Employee, error) {
	// Проверяем существование департамента
	dept, err := s.deptRepo.GetByID(ctx, deptID)
	if err != nil || dept == nil {
		return nil, errors.New("department not found")
	}
	fullName, err := validateString(input.FullName, 200)
	if err != nil {
		return nil, err
	}
	position, err := validateString(input.Position, 200)
	if err != nil {
		return nil, err
	}
	emp := &models.Employee{
		DepartmentID: deptID,
		FullName:     fullName,
		Position:     position,
		HiredAt:      input.HiredAt,
	}
	err = s.empRepo.Create(ctx, emp)
	return emp, err
}
