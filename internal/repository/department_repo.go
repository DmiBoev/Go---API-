package repository

import (
	"context"
	"dept-api/internal/models"
	"errors"

	"gorm.io/gorm"
)

type DepartmentRepository interface {
	Create(ctx context.Context, dept *models.Department) error
	GetByID(ctx context.Context, id uint) (*models.Department, error)
	Update(ctx context.Context, dept *models.Department) error
	Delete(ctx context.Context, id uint) error
	GetChildren(ctx context.Context, parentID uint) ([]models.Department, error)
	GetChildrenRecursive(ctx context.Context, parentID uint, depth int, includeEmployees bool) (*models.Department, error)
	CheckCyclicMove(ctx context.Context, deptID, newParentID uint) (bool, error)
	GetEmployeesByDepartment(ctx context.Context, deptID uint) ([]models.Employee, error)
	ReassignEmployees(ctx context.Context, fromDeptID, toDeptID uint) error
	ReassignChildrenParent(ctx context.Context, oldParentID, newParentID *uint) error // для reassign: поднять детей на уровень выше
}

type departmentRepository struct {
	db *gorm.DB
}

func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepository{db: db}
}

func (r *departmentRepository) Create(ctx context.Context, dept *models.Department) error {
	return r.db.WithContext(ctx).Create(dept).Error
}

func (r *departmentRepository) GetByID(ctx context.Context, id uint) (*models.Department, error) {
	var dept models.Department
	err := r.db.WithContext(ctx).First(&dept, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &dept, err
}

func (r *departmentRepository) Update(ctx context.Context, dept *models.Department) error {
	return r.db.WithContext(ctx).Save(dept).Error
}

func (r *departmentRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Department{}, id).Error
}

func (r *departmentRepository) GetChildren(ctx context.Context, parentID uint) ([]models.Department, error) {
	var children []models.Department
	err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("id asc").Find(&children).Error
	return children, err
}

// GetChildrenRecursive строит дерево до определённой глубины
func (r *departmentRepository) GetChildrenRecursive(ctx context.Context, parentID uint, depth int, includeEmployees bool) (*models.Department, error) {
	var root models.Department
	if err := r.db.WithContext(ctx).First(&root, parentID).Error; err != nil {
		return nil, err
	}
	if includeEmployees {
		r.db.Where("department_id = ?", parentID).Order("created_at asc").Find(&root.Employees)
	}
	if depth > 1 {
		children, err := r.GetChildren(ctx, parentID)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			subTree, err := r.GetChildrenRecursive(ctx, child.ID, depth-1, includeEmployees)
			if err != nil {
				continue
			}
			root.Children = append(root.Children, *subTree)
		}
	}
	return &root, nil
}

// CheckCyclicMove проверяет, не создаст ли перемещение цикла
func (r *departmentRepository) CheckCyclicMove(ctx context.Context, deptID, newParentID uint) (bool, error) {
	var exists bool
	query := `
        WITH RECURSIVE ancestors AS (
            SELECT parent_id FROM departments WHERE id = ?
            UNION ALL
            SELECT d.parent_id FROM departments d
            INNER JOIN ancestors a ON d.id = a.parent_id
        )
        SELECT EXISTS(SELECT 1 FROM ancestors WHERE parent_id = ?)
    `
	err := r.db.WithContext(ctx).Raw(query, newParentID, deptID).Scan(&exists).Error
	return exists, err
}

func (r *departmentRepository) GetEmployeesByDepartment(ctx context.Context, deptID uint) ([]models.Employee, error) {
	var employees []models.Employee
	err := r.db.WithContext(ctx).Where("department_id = ?", deptID).Order("created_at asc").Find(&employees).Error
	return employees, err
}

func (r *departmentRepository) ReassignEmployees(ctx context.Context, fromDeptID, toDeptID uint) error {
	return r.db.WithContext(ctx).Model(&models.Employee{}).Where("department_id = ?", fromDeptID).Update("department_id", toDeptID).Error
}

// ReassignChildrenParent поднимает детей удаляемого подразделения на уровень его родителя
func (r *departmentRepository) ReassignChildrenParent(ctx context.Context, oldParentID, newParentID *uint) error {
	return r.db.WithContext(ctx).Model(&models.Department{}).Where("parent_id = ?", oldParentID).Update("parent_id", newParentID).Error
}
