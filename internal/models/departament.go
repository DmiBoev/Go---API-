package models

import (
	"time"
)

type Department struct {
	ID        uint         `db:"id" json:"id"`
	Name      string       `db:"name" json:"name"`
	ParentID  *uint        `db:"parent_id" json:"parent_id"`
	CreatedAt time.Time    `db:"created_at" json:"created_at"`
	Employees []Employee   `db:"-" json:"employees,omitempty"`
	Children  []Department `db:"-" json:"children,omitempty"`
}

// DTO для создания подразделения
type CreateDepartmentInput struct {
	Name     string `db:"name" json:"name"`
	ParentID *uint  `db:"parent_id" json:"parent_id"`
}

// DTO для обновления подразделения
type UpdateDepartmentInput struct {
	Name     *string `db:"name" json:"name"`
	ParentID *uint   `db:"parent_id" json:"parent_id"`
}
