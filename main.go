package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ============================================================================
// МОДЕЛИ
// ============================================================================

type Department struct {
	ID        uint         `gorm:"primaryKey" json:"id"`
	Name      string       `gorm:"type:varchar(200);not null" json:"name"`
	ParentID  *uint        `gorm:"index" json:"parent_id"`
	CreatedAt time.Time    `json:"created_at"`
	Employees []Employee   `gorm:"foreignKey:DepartmentID;constraint:OnDelete:CASCADE" json:"employees,omitempty"`
	Children  []Department `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

type Employee struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DepartmentID uint       `gorm:"index;not null" json:"department_id"`
	FullName     string     `gorm:"type:varchar(200);not null" json:"full_name"`
	Position     string     `gorm:"type:varchar(200);not null" json:"position"`
	HiredAt      *time.Time `gorm:"type:date" json:"hired_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ============================================================================
// DTO
// ============================================================================

type CreateDeptInput struct {
	Name     string `json:"name"`
	ParentID *uint  `json:"parent_id"`
}

type UpdateDeptInput struct {
	Name     *string `json:"name"`
	ParentID *uint   `json:"parent_id"`
}

type CreateEmpInput struct {
	FullName string     `json:"full_name"`
	Position string     `json:"position"`
	HiredAt  *time.Time `json:"hired_at"`
}

// ============================================================================
// APP
// ============================================================================

type App struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// ============================================================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ============================================================================

func (app *App) respondWithError(w http.ResponseWriter, code int, message string) {
	app.Logger.Error("API Error", slog.Int("code", code), slog.String("msg", message))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (app *App) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	return false
}

func validateString(field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" || len(trimmed) > maxLen {
		return "", fmt.Errorf("длина должна быть от 1 до %d символов", maxLen)
	}
	return trimmed, nil
}

func (app *App) buildTree(deptID uint, currentDepth, maxDepth int, includeEmp bool) (*Department, error) {
	var dept Department
	if err := app.DB.First(&dept, deptID).Error; err != nil {
		return nil, err
	}
	if includeEmp {
		// сортировка по умолчанию: created_at asc, можно расширить параметром
		app.DB.Where("department_id = ?", deptID).Order("created_at asc").Find(&dept.Employees)
	}
	if currentDepth < maxDepth {
		var children []Department
		app.DB.Where("parent_id = ?", deptID).Order("id asc").Find(&children)
		for _, child := range children {
			childTree, err := app.buildTree(child.ID, currentDepth+1, maxDepth, includeEmp)
			if err == nil && childTree != nil {
				dept.Children = append(dept.Children, *childTree)
			}
		}
	}
	return &dept, nil
}

// ============================================================================
// ХЕНДЛЕРЫ
// ============================================================================

// 1. Создать подразделение
func (app *App) handleCreateDepartment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var input CreateDeptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	name, err := validateString(input.Name, 200)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Name: "+err.Error())
		return
	}

	if input.ParentID != nil {
		var parent Department
		if err := app.DB.WithContext(r.Context()).First(&parent, *input.ParentID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "Parent department not found")
			return
		}
	}

	dept := Department{Name: name, ParentID: input.ParentID}
	result := app.DB.WithContext(r.Context()).Create(&dept)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			app.respondWithError(w, http.StatusBadRequest, "Department with this name already exists under the same parent")
			return
		}
		app.respondWithError(w, http.StatusInternalServerError, "Failed to create department")
		return
	}

	app.respondWithJSON(w, http.StatusCreated, dept)
}

// 2. Создать сотрудника в подразделении
func (app *App) handleCreateEmployee(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}

	var dept Department
	if err := app.DB.WithContext(r.Context()).First(&dept, uint(deptID)).Error; err != nil {
		app.respondWithError(w, http.StatusNotFound, "Department not found")
		return
	}

	var input CreateEmpInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	fullName, err := validateString(input.FullName, 200)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "full_name: "+err.Error())
		return
	}
	position, err := validateString(input.Position, 200)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "position: "+err.Error())
		return
	}

	emp := Employee{
		DepartmentID: dept.ID,
		FullName:     fullName,
		Position:     position,
		HiredAt:      input.HiredAt,
	}
	app.DB.WithContext(r.Context()).Create(&emp)
	app.respondWithJSON(w, http.StatusCreated, emp)
}

// 3. Получить подразделение (дерево)
func (app *App) handleGetDepartment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}

	depth := 1
	if dStr := r.URL.Query().Get("depth"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d >= 1 && d <= 5 {
			depth = d
		}
	}
	includeEmp := r.URL.Query().Get("include_employees") != "false"

	tree, err := app.buildTree(uint(deptID), 1, depth, includeEmp)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app.respondWithError(w, http.StatusNotFound, "Department not found")
		} else {
			app.respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	app.respondWithJSON(w, http.StatusOK, tree)
}

// 4. Обновить подразделение (переместить, переименовать)
func (app *App) handleUpdateDepartment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	uid := uint(deptID)

	var dept Department
	if err := app.DB.WithContext(r.Context()).First(&dept, uid).Error; err != nil {
		app.respondWithError(w, http.StatusNotFound, "Department not found")
		return
	}

	var input UpdateDeptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	// Валидация имени
	if input.Name != nil {
		trimmed, err := validateString(*input.Name, 200)
		if err != nil {
			app.respondWithError(w, http.StatusBadRequest, "Name: "+err.Error())
			return
		}
		input.Name = &trimmed
	}

	// Проверка parent_id на циклы и существование
	if input.ParentID != nil {
		if *input.ParentID == uid {
			app.respondWithError(w, http.StatusBadRequest, "Department cannot be its own parent")
			return
		}
		var parent Department
		if err := app.DB.WithContext(r.Context()).First(&parent, *input.ParentID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "New parent not found")
			return
		}
		// Проверка цикла (нельзя переместить внутрь своего потомка)
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
		if err := app.DB.Raw(query, *input.ParentID, uid).Scan(&exists).Error; err != nil {
			app.respondWithError(w, http.StatusInternalServerError, "Cycle check failed")
			return
		}
		if exists {
			app.respondWithError(w, http.StatusConflict, "Cannot move department inside its own descendant")
			return
		}
	}

	// Применяем изменения в транзакции (чтобы избежать race condition)
	err = app.DB.Transaction(func(tx *gorm.DB) error {
		if input.Name != nil {
			if err := tx.Model(&dept).Update("name", *input.Name).Error; err != nil {
				if isDuplicateKeyError(err) {
					return fmt.Errorf("duplicate name under same parent")
				}
				return err
			}
		}
		if input.ParentID != nil {
			if err := tx.Model(&dept).Update("parent_id", input.ParentID).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		if err.Error() == "duplicate name under same parent" {
			app.respondWithError(w, http.StatusBadRequest, "Department with this name already exists under the same parent")
		} else {
			app.respondWithError(w, http.StatusInternalServerError, "Failed to update department")
		}
		return
	}

	// Перезагружаем запись
	app.DB.WithContext(r.Context()).First(&dept, uid)
	app.respondWithJSON(w, http.StatusOK, dept)
}

// 5. Удалить подразделение (cascade / reassign)
func (app *App) handleDeleteDepartment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	uid := uint(deptID)

	var dept Department
	if err := app.DB.WithContext(r.Context()).First(&dept, uid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app.respondWithError(w, http.StatusNotFound, "Department not found")
		} else {
			app.respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode != "cascade" && mode != "reassign" {
		app.respondWithError(w, http.StatusBadRequest, "Mode must be 'cascade' or 'reassign'")
		return
	}

	if mode == "reassign" {
		targetStr := r.URL.Query().Get("reassign_to_department_id")
		if targetStr == "" {
			app.respondWithError(w, http.StatusBadRequest, "reassign_to_department_id is required for reassign mode")
			return
		}
		targetID, err := strconv.ParseUint(targetStr, 10, 32)
		if err != nil || targetID == 0 {
			app.respondWithError(w, http.StatusBadRequest, "Invalid reassign target ID")
			return
		}
		if uint(targetID) == uid {
			app.respondWithError(w, http.StatusBadRequest, "Cannot reassign to the same department")
			return
		}
		var targetDept Department
		if err := app.DB.WithContext(r.Context()).First(&targetDept, targetID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "Reassign target department not found")
			return
		}

		// Транзакция: переводим сотрудников и дочерние подразделения
		err = app.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&Employee{}).Where("department_id = ?", uid).Update("department_id", targetID).Error; err != nil {
				return err
			}
			if err := tx.Model(&Department{}).Where("parent_id = ?", uid).Update("parent_id", dept.ParentID).Error; err != nil {
				return err
			}
			if err := tx.Delete(&dept).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			app.respondWithError(w, http.StatusInternalServerError, "Failed to reassign and delete: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// mode == "cascade" – достаточно удалить родителя, БД удалит всё остальное по ON DELETE CASCADE
	if err := app.DB.WithContext(r.Context()).Delete(&dept).Error; err != nil {
		app.respondWithError(w, http.StatusInternalServerError, "Failed to delete department: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=secretpassword dbname=department_management port=5432 sslmode=disable"
	}

	// Отдельное соединение для миграций (goose)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Error("Failed to open SQL DB for migrations", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("Goose failed to set dialect", "error", err)
		os.Exit(1)
	}

	logger.Info("Running migrations...")
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		logger.Error("Migrations failed", "error", err)
		os.Exit(1)
	}

	// GORM соединение
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database via GORM", "error", err)
		os.Exit(1)
	}

	app := &App{DB: gormDB, Logger: logger}

	// Роутер (Go 1.22+)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", app.handleCreateDepartment)
	mux.HandleFunc("POST /departments/{id}/employees/", app.handleCreateEmployee)
	mux.HandleFunc("GET /departments/{id}", app.handleGetDepartment)
	mux.HandleFunc("PATCH /departments/{id}", app.handleUpdateDepartment)
	mux.HandleFunc("DELETE /departments/{id}", app.handleDeleteDepartment)

	// Middleware логирования
	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("Request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}

	port := ":8080"
	logger.Info("Server starting", slog.String("port", port))
	if err := http.ListenAndServe(port, loggingMiddleware(mux)); err != nil {
		logger.Error("Server shutdown", "error", err)
	}
}
