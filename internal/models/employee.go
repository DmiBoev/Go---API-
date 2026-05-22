package models

import (
	"time"
)

type Employee struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DepartmentID uint       `gorm:"index;not null" json:"department_id"`
	FullName     string     `gorm:"type:varchar(200);not null" json:"full_name"`
	Position     string     `gorm:"type:varchar(200);not null" json:"position"`
	HiredAt      *time.Time `gorm:"type:date" json:"hired_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateEmployeeInput struct {
	FullName string     `json:"full_name"`
	Position string     `json:"position"`
	HiredAt  *time.Time `json:"hired_at"`
}
