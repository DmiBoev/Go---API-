package models

import (
	"time"
)

type Department struct {
	ID        uint         `gorm:"primaryKey" json:"id"`
	Name      string       `gorm:"type:varchar(200);not null" json:"name"`
	ParentID  *uint        `gorm:"index" json:"parent_id"`
	CreatedAt time.Time    `json:"created_at"`
	Employees []Employee   `gorm:"foreignKey:DepartmentID;constraint:OnDelete:CASCADE" json:"employees,omitempty"`
	Children  []Department `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

// DTO для создания подразделения
type CreateDepartmentInput struct {
	Name     string `json:"name"`
	ParentID *uint  `json:"parent_id"`
}

// DTO для обновления подразделения
type UpdateDepartmentInput struct {
	Name     *string `json:"name"`
	ParentID *uint   `json:"parent_id"`
}
