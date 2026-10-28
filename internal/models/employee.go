package models

import (
	"time"
)

type Employee struct {
	ID           uint       `db:"id" json:"id"`
	DepartmentID uint       `db:"department_id" json:"department_id"`
	FullName     string     `db:"full_name" json:"full_name"`
	Position     string     `db:"position" json:"position"`
	HiredAt      *time.Time `db:"hired_at" json:"hired_at"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}

type CreateEmployeeInput struct {
	FullName string     `db:"full_name" json:"full_name"`
	Position string     `db:"position" json:"position"`
	HiredAt  *time.Time `db:"hired_at" json:"hired_at"`
}
