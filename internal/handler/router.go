package handler

import "net/http"

// NewRouter is shared by the production server and HTTP integration tests.
func NewRouter(h *DepartmentHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /departments/{$}", h.CreateDepartment)
	mux.HandleFunc("POST /departments/{id}/employees/{$}", h.CreateEmployee)
	mux.HandleFunc("GET /departments/{id}", h.GetDepartment)
	mux.HandleFunc("PATCH /departments/{id}", h.UpdateDepartment)
	mux.HandleFunc("DELETE /departments/{id}", h.DeleteDepartment)
	return mux
}
