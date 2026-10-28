package repository

import (
	"context"
	"database/sql"
	"dept-api/internal/models"
	"errors"

	"github.com/jmoiron/sqlx"
)

type EmployeeRepository interface {
	Create(context.Context, *models.Employee) error
	GetByID(context.Context, uint) (*models.Employee, error)
	Update(context.Context, *models.Employee) error
	Delete(context.Context, uint) error
}

type employeeRepository struct{ db sqlx.ExtContext }

func NewEmployeeRepository(db sqlx.ExtContext) EmployeeRepository {
	return &employeeRepository{db: db}
}

const employeeColumns = "id, department_id, full_name, position, hired_at, created_at"

func (r *employeeRepository) Create(ctx context.Context, emp *models.Employee) error {
	return databaseError(sqlx.GetContext(ctx, r.db, emp, `INSERT INTO employees (department_id, full_name, position, hired_at)
  VALUES ($1, $2, $3, $4) RETURNING `+employeeColumns, emp.DepartmentID, emp.FullName, emp.Position, emp.HiredAt))
}

func (r *employeeRepository) GetByID(ctx context.Context, id uint) (*models.Employee, error) {
	var emp models.Employee
	err := sqlx.GetContext(ctx, r.db, &emp, "SELECT "+employeeColumns+" FROM employees WHERE id = $1", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &emp, nil
}

func (r *employeeRepository) Update(ctx context.Context, emp *models.Employee) error {
	return databaseError(sqlx.GetContext(ctx, r.db, emp, `UPDATE employees
  SET department_id = $1, full_name = $2, position = $3, hired_at = $4 WHERE id = $5 RETURNING `+employeeColumns,
		emp.DepartmentID, emp.FullName, emp.Position, emp.HiredAt, emp.ID))
}

func (r *employeeRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM employees WHERE id = $1", id)
	return databaseError(err)
}
