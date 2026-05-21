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

	"://github.com"
	_ "://github.com"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ==============================================================================
// МОДЕЛИ И СТРУКТУРЫ
// ==============================================================================

type Department struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"type:varchar(200);not null" json:"name"`
	ParentID  *uint          `gorm:"index" json:"parent_id"`
	CreatedAt time.Time      `json:"created_at"`
	Employees []Employee     `gorm:"foreignKey:DepartmentID;constraint:OnDelete:CASCADE" json:"employees,omitempty"`
	Children  []Department   `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

type Employee struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DepartmentID uint       `gorm:"index;not null" json:"department_id"`
	FullName     string     `gorm:"type:varchar(200);not null" json:"full_name"`
	Position     string     `gorm:"type:varchar(200);not null" json:"position"`
	HiredAt      *time.Time `gorm:"type:date" json:"hired_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

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

type App struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// ==============================================================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ==============================================================================

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

func (app *App) checkNameUniqueness(name string, parentID *uint, excludeID *uint) error {
	var count int64
	query := app.DB.Model(&Department{}).Where("name = ?", name)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	query.Count(&count)
	if count > 0 {
		return fmt.Errorf("подразделение с именем '%s' уже существует в этом узле", name)
	}
	return nil
}

func (app *App) getAllDescendantIDs(deptID uint) ([]uint, error) {
	var ids []uint
	query := `
		WITH RECURSIVE descendants AS (
			SELECT id FROM departments WHERE parent_id = ?
			UNION ALL
			SELECT d.id FROM departments d
			INNER JOIN descendants desc ON d.parent_id = desc.id
		)
		SELECT id FROM descendants;
	`
	err := app.DB.Raw(query, deptID).Scan(&ids).Error
	return ids, err
}

func (app *App) buildTree(deptID uint, currentDepth, maxDepth int, includeEmp bool) (*Department, error) {
	var dept Department
	if err := app.DB.First(&dept, deptID).Error; err != nil {
		return nil, err
	}
	if includeEmp {
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

// ==============================================================================
// ХЕНДЛЕРЫ МАРШРУТОВ
// ==============================================================================

func (app *App) handleCreateDepartment(w http.ResponseWriter, r *http.Request) {
	var input CreateDeptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 200 {
		app.respondWithError(w, http.StatusBadRequest, "Name must be between 1 and 200 characters")
		return
	}

	if input.ParentID != nil {
		var parent Department
		if err := app.DB.First(&parent, *input.ParentID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "Parent department not found")
			return
		}
	}

	if err := app.checkNameUniqueness(name, input.ParentID, nil); err != nil {
		app.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	dept := Department{Name: name, ParentID: input.ParentID}
	if err := app.DB.Create(&dept).Error; err != nil {
		app.respondWithError(w, http.StatusInternalServerError, "Failed to create department")
		return
	}
	app.respondWithJSON(w, http.StatusCreated, dept)
}

func (app *App) handleCreateEmployee(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, _ := strconv.Atoi(idStr)

	var dept Department
	if err := app.DB.First(&dept, deptID).Error; err != nil {
		app.respondWithError(w, http.StatusNotFound, "Department not found")
		return
	}

	var input CreateEmpInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	fullName := strings.TrimSpace(input.FullName)
	position := strings.TrimSpace(input.Position)
	if fullName == "" || len(fullName) > 200 || position == "" || len(position) > 200 {
		app.respondWithError(w, http.StatusBadRequest, "Fields must be between 1 and 200 characters")
		return
	}

	emp := Employee{
		DepartmentID: dept.ID,
		FullName:     fullName,
		Position:     position,
		HiredAt:      input.HiredAt,
	}
	app.DB.Create(&emp)
	app.respondWithJSON(w, http.StatusCreated, emp)
}

func (app *App) handleGetDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, _ := strconv.Atoi(idStr)

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

func (app *App) handleUpdateDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)
	deptID := uint(id)

	var dept Department
	if err := app.DB.First(&dept, deptID).Error; err != nil {
		app.respondWithError(w, http.StatusNotFound, "Department not found")
		return
	}

	var input UpdateDeptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		app.respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" || len(trimmed) > 200 {
			app.respondWithError(w, http.StatusBadRequest, "Name cannot be empty or > 200 chars")
			return
		}
		input.Name = &trimmed
	}

	if input.ParentID != nil {
		if *input.ParentID == deptID {
			app.respondWithError(w, http.StatusBadRequest, "Department cannot be its own parent")
			return
		}
		var parent Department
		if err := app.DB.First(&parent, *input.ParentID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "New parent not found")
			return
		}
		descendants, _ := app.getAllDescendantIDs(deptID)
		for _, dID := range descendants {
			if *input.ParentID == dID {
				app.respondWithError(w, http.StatusConflict, "Tree cycle detected")
				return
			}
		}
	}

	finalName := dept.Name
	if input.Name != nil {
		finalName = *input.Name
	}
	finalParentID := dept.ParentID
	if input.ParentID != nil {
		finalParentID = input.ParentID
	}

	if err := app.checkNameUniqueness(finalName, finalParentID, &deptID); err != nil {
		app.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Name != nil {
		dept.Name = *input.Name
	}
	if input.ParentID != nil {
		dept.ParentID = input.ParentID
	}
	app.DB.Save(&dept)
	app.respondWithJSON(w, http.StatusOK, dept)
}

func (app *App) handleDeleteDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)
	deptID := uint(id)

	var dept Department
	if err := app.DB.First(&dept, deptID).Error; err != nil {
		app.respondWithError(w, http.StatusNotFound, "Department not found")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode != "cascade" && mode != "reassign" {
		app.respondWithError(w, http.StatusBadRequest, "Mode must be 'cascade' or 'reassign'")
		return
	}

	if mode == "reassign" {
		targetStr := r.URL.Query().Get("reassign_to_department_id")
		targetID, _ := strconv.Atoi(targetStr)
		if targetID == 0 || uint(targetID) == deptID {
			app.respondWithError(w, http.StatusBadRequest, "Invalid reassign target ID")
			return
		}
		var targetDept Department
		if err := app.DB.First(&targetDept, targetID).Error; err != nil {
			app.respondWithError(w, http.StatusNotFound, "Reassign target department not found")
			return
		}

		app.DB.Model(&Employee{}).Where("department_id = ?", deptID).Update("department_id", targetID)
		app.DB.Model(&Department{}).Where("parent_id = ?", deptID).Update("parent_id", dept.ParentID)
	}

	if mode == "cascade" {
		descendants, _ := app.getAllDescendantIDs(deptID)
		if len(descendants) > 0 {
			app.DB.Where("id IN ?", descendants).Delete(&Department{})
		}
	}

	app.DB.Delete(&dept)
	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// ЗАПУСК И ИНИЦИАЛИЗАЦИЯ
// ==============================================================================

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=dept_db port=5432 sslmode=disable"
	}

	// Выполнение миграций через Goose (чистый sql драйвер)
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

	// Инициализация GORM
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database via GORM", "error", err)
		os.Exit(1)
	}

	app := &App{DB: gormDB, Logger: logger}

	// Встроенный роутер Go 1.22+
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/", app.handleCreateDepartment)
	mux.HandleFunc("POST /departments/{id}/employees/", app.handleCreateEmployee)
	mux.HandleFunc("GET /departments/{id}", app.handleGetDepartment)
	mux.HandleFunc("PATCH /departments/{id}", app.handleUpdateDepartment)
	mux.HandleFunc("DELETE /departments/{id}", app.handleDeleteDepartment)

	// Простой Логгинг-мидлвар
	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("Request handled",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}

	port := ":8080"
	logger.Info("Server is running on port " + port)
	if err := http.ListenAndServe(port, loggingMiddleware(mux)); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}
}
