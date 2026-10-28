package repository

import (
	"context"
	"database/sql"
	"dept-api/internal/domain"
	"dept-api/internal/models"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type DepartmentRepository interface {
	Create(context.Context, *models.Department) error
	GetByID(context.Context, uint) (*models.Department, error)
	Update(context.Context, *models.Department) error
	Delete(context.Context, uint) error
	GetChildren(context.Context, uint) ([]models.Department, error)
	GetChildrenRecursive(context.Context, uint, int, bool) (*models.Department, error)
	CheckCyclicMove(context.Context, uint, uint) (bool, error)
	GetEmployeesByDepartment(context.Context, uint) ([]models.Employee, error)
	ReassignEmployees(context.Context, uint, uint) error
	ReassignChildrenParent(context.Context, *uint, *uint) error
	WithinTransaction(context.Context, func(DepartmentRepository) error) error
}

type departmentRepository struct{ db sqlx.ExtContext }

func NewDepartmentRepository(db sqlx.ExtContext) DepartmentRepository {
	return &departmentRepository{db: db}
}

// Every callback query uses the same transaction. The lock serializes hierarchy
// mutations so concurrent moves cannot create a cycle.
func (r *departmentRepository) WithinTransaction(ctx context.Context, fn func(DepartmentRepository) error) error {
	db, ok := r.db.(*sqlx.DB)
	if !ok {
		return errors.New("nested department transactions are not supported")
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "LOCK TABLE departments IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return err
	}
	if err := fn(NewDepartmentRepository(tx)); err != nil {
		return err
	}
	return databaseError(tx.Commit())
}

const departmentColumns = "id, name, parent_id, created_at"

func (r *departmentRepository) Create(ctx context.Context, dept *models.Department) error {
	return databaseError(sqlx.GetContext(ctx, r.db, dept,
		"INSERT INTO departments (name, parent_id) VALUES ($1, $2) RETURNING "+departmentColumns, dept.Name, dept.ParentID))
}

func (r *departmentRepository) GetByID(ctx context.Context, id uint) (*models.Department, error) {
	var dept models.Department
	err := sqlx.GetContext(ctx, r.db, &dept, "SELECT "+departmentColumns+" FROM departments WHERE id = $1", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *departmentRepository) Update(ctx context.Context, dept *models.Department) error {
	return databaseError(sqlx.GetContext(ctx, r.db, dept,
		"UPDATE departments SET name = $1, parent_id = $2 WHERE id = $3 RETURNING "+departmentColumns,
		dept.Name, dept.ParentID, dept.ID))
}

func (r *departmentRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM departments WHERE id = $1", id)
	return databaseError(err)
}

func (r *departmentRepository) GetChildren(ctx context.Context, parentID uint) ([]models.Department, error) {
	children := make([]models.Department, 0)
	err := sqlx.SelectContext(ctx, r.db, &children, "SELECT "+departmentColumns+" FROM departments WHERE parent_id = $1 ORDER BY id", parentID)
	return children, err
}

func (r *departmentRepository) GetChildrenRecursive(ctx context.Context, id uint, depth int, includeEmployees bool) (*models.Department, error) {
	root, err := r.GetByID(ctx, id)
	if err != nil || root == nil {
		return root, err
	}
	if includeEmployees {
		root.Employees, err = r.GetEmployeesByDepartment(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	if depth > 1 {
		children, err := r.GetChildren(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			subtree, err := r.GetChildrenRecursive(ctx, child.ID, depth-1, includeEmployees)
			if err != nil {
				return nil, err
			}
			if subtree != nil {
				root.Children = append(root.Children, *subtree)
			}
		}
	}
	return root, nil
}

func (r *departmentRepository) CheckCyclicMove(ctx context.Context, deptID, newParentID uint) (bool, error) {
	var exists bool
	err := sqlx.GetContext(ctx, r.db, &exists, `WITH RECURSIVE ancestors AS (
  SELECT id, parent_id FROM departments WHERE id = $1
  UNION
  SELECT d.id, d.parent_id FROM departments d JOIN ancestors a ON d.id = a.parent_id
 ) SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = $2)`, newParentID, deptID)
	return exists, err
}

func (r *departmentRepository) GetEmployeesByDepartment(ctx context.Context, id uint) ([]models.Employee, error) {
	employees := make([]models.Employee, 0)
	err := sqlx.SelectContext(ctx, r.db, &employees, "SELECT "+employeeColumns+" FROM employees WHERE department_id = $1 ORDER BY created_at, id", id)
	return employees, err
}

func (r *departmentRepository) ReassignEmployees(ctx context.Context, from, to uint) error {
	_, err := r.db.ExecContext(ctx, "UPDATE employees SET department_id = $1 WHERE department_id = $2", to, from)
	return databaseError(err)
}

func (r *departmentRepository) ReassignChildrenParent(ctx context.Context, oldParent, newParent *uint) error {
	_, err := r.db.ExecContext(ctx, "UPDATE departments SET parent_id = $1 WHERE parent_id = $2", newParent, oldParent)
	return databaseError(err)
}

func databaseError(err error) error {
	var pgErr *pq.Error
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.ErrNotFound
	case errors.As(err, &pgErr):
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%w: department name already exists under this parent", domain.ErrConflict)
		case "23503":
			return fmt.Errorf("%w: referenced department does not exist", domain.ErrInvalid)
		}
	}
	return err
}
