package handler

import (
	"dept-api/internal/models"
	"dept-api/internal/service"
	"encoding/json"
	"net/http"
	"strconv"
)

type DepartmentHandler struct {
	deptService service.DepartmentService
	empService  service.EmployeeService
}

func NewDepartmentHandler(deptService service.DepartmentService, empService service.EmployeeService) *DepartmentHandler {
	return &DepartmentHandler{
		deptService: deptService,
		empService:  empService,
	}
}

// respondWithError – утилита
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *DepartmentHandler) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	var input models.CreateDepartmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	dept, err := h.deptService.Create(r.Context(), input)
	if err != nil {
		// Обработка ошибки уникальности можно по строке сообщения, но проще: если ошибка содержит "duplicate" - 400
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusCreated, dept)
}

func (h *DepartmentHandler) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	var input models.CreateEmployeeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	emp, err := h.empService.Create(r.Context(), uint(deptID), input)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJSON(w, http.StatusCreated, emp)
}

func (h *DepartmentHandler) GetDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	depth := 1
	if dStr := r.URL.Query().Get("depth"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d >= 1 && d <= 5 {
			depth = d
		}
	}
	includeEmployees := r.URL.Query().Get("include_employees") != "false"
	dept, err := h.deptService.GetByID(r.Context(), uint(deptID), depth, includeEmployees)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, dept)
}

func (h *DepartmentHandler) UpdateDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	var input models.UpdateDepartmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	dept, err := h.deptService.Update(r.Context(), uint(deptID), input)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "department not found" {
			status = http.StatusNotFound
		} else if err.Error() == "department cannot be its own parent" || err.Error() == "new parent not found" {
			status = http.StatusBadRequest
		} else if err.Error() == "cannot move department inside its own descendant" {
			status = http.StatusConflict
		}
		respondWithError(w, status, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, dept)
}

func (h *DepartmentHandler) DeleteDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deptID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid department ID")
		return
	}
	mode := r.URL.Query().Get("mode")
	var reassignTo *uint
	if mode == "reassign" {
		reassignStr := r.URL.Query().Get("reassign_to_department_id")
		if reassignStr == "" {
			respondWithError(w, http.StatusBadRequest, "reassign_to_department_id required")
			return
		}
		id, err := strconv.ParseUint(reassignStr, 10, 32)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid reassign target")
			return
		}
		reassignTo = new(uint)
		*reassignTo = uint(id)
	}
	err = h.deptService.Delete(r.Context(), uint(deptID), mode, reassignTo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
