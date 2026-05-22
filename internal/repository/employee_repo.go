package repository

import (
	"context"
	"dept-api/internal/models"

	"gorm.io/gorm"
)

type EmployeeRepository interface {
	Create(ctx context.Context, emp *models.Employee) error
	GetByID(ctx context.Context, id uint) (*models.Employee, error)
	Update(ctx context.Context, emp *models.Employee) error
	Delete(ctx context.Context, id uint) error
}

type employeeRepository struct {
	db *gorm.DB
}

func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepository{db: db}
}

func (r *employeeRepository) Create(ctx context.Context, emp *models.Employee) error {
	return r.db.WithContext(ctx).Create(emp).Error
}

func (r *employeeRepository) GetByID(ctx context.Context, id uint) (*models.Employee, error) {
	var emp models.Employee
	err := r.db.WithContext(ctx).First(&emp, id).Error
	return &emp, err
}

func (r *employeeRepository) Update(ctx context.Context, emp *models.Employee) error {
	return r.db.WithContext(ctx).Save(emp).Error
}

func (r *employeeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Employee{}, id).Error
}
