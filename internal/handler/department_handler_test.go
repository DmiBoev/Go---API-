//go:build integration

package handler

import (
	"bytes"
	"dept-api/internal/middleware"
	"dept-api/internal/models"
	"dept-api/internal/repository"
	"dept-api/internal/service"
	"dept-api/internal/testutil"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { os.Exit(testutil.Run(m)) }

type apiTest struct {
	t      *testing.T
	db     *sqlx.DB
	server *httptest.Server
}

func setupAPI(t *testing.T) *apiTest {
	t.Helper()
	db := testutil.NewDB(t)
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	h := NewDepartmentHandler(service.NewDepartmentService(deptRepo), service.NewEmployeeService(empRepo, deptRepo))
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := httptest.NewServer(middleware.Wrap(logger, []string{"https://client.example"}, NewRouter(h)))
	t.Cleanup(server.Close)
	return &apiTest{t: t, db: db, server: server}
}

func (a *apiTest) request(method, path, body string, status int) []byte {
	a.t.Helper()
	req, err := http.NewRequest(method, a.server.URL+path, bytes.NewBufferString(body))
	require.NoError(a.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://client.example")
	res, err := a.server.Client().Do(req)
	require.NoError(a.t, err)
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	require.NoError(a.t, err)
	require.Equal(a.t, status, res.StatusCode, "response: %s", data)
	assert.Equal(a.t, "https://client.example", res.Header.Get("Access-Control-Allow-Origin"))
	return data
}

func (a *apiTest) department(name string, parent *uint) models.Department {
	a.t.Helper()
	body, err := json.Marshal(models.CreateDepartmentInput{Name: name, ParentID: parent})
	require.NoError(a.t, err)
	var dept models.Department
	require.NoError(a.t, json.Unmarshal(a.request("POST", "/departments/", string(body), 201), &dept))
	require.NotZero(a.t, dept.ID)
	require.False(a.t, dept.CreatedAt.IsZero())
	return dept
}

func (a *apiTest) employee(dept uint) models.Employee {
	a.t.Helper()
	var emp models.Employee
	data := a.request("POST", fmt.Sprintf("/departments/%d/employees/", dept), `{"full_name":"Иван Петров","position":"Developer","hired_at":"2023-01-15T00:00:00Z"}`, 201)
	require.NoError(a.t, json.Unmarshal(data, &emp))
	require.NotNil(a.t, emp.HiredAt)
	assert.Equal(a.t, "2023-01-15", emp.HiredAt.Format("2006-01-02"))
	assert.Equal(a.t, dept, emp.DepartmentID)
	return emp
}

func TestDepartmentAPIHierarchy(t *testing.T) {
	a := setupAPI(t)
	root := a.department("Root", nil)
	child := a.department("Child", &root.ID)
	grandchild := a.department("Grandchild", &child.ID)
	a.employee(child.ID)
	var tree models.Department
	data := a.request("GET", fmt.Sprintf("/departments/%d?depth=3", root.ID), "", 200)
	require.NoError(t, json.Unmarshal(data, &tree))
	require.Len(t, tree.Children, 1)
	require.Len(t, tree.Children[0].Children, 1)
	assert.Equal(t, grandchild.ID, tree.Children[0].Children[0].ID)
	require.Len(t, tree.Children[0].Employees, 1)
	data = a.request("GET", fmt.Sprintf("/departments/%d?depth=1", root.ID), "", 200)
	require.NoError(t, json.Unmarshal(data, &tree)) // decode into a fresh value below for omitted arrays
	var shallow models.Department
	require.NoError(t, json.Unmarshal(data, &shallow))
	assert.Empty(t, shallow.Children)
	data = a.request("GET", fmt.Sprintf("/departments/%d?depth=3&include_employees=false", root.ID), "", 200)
	assert.NotContains(t, string(data), `"employees"`)
	a.request("PATCH", fmt.Sprintf("/departments/%d", child.ID), `{"name":"Renamed"}`, 200)
	a.request("PATCH", fmt.Sprintf("/departments/%d", root.ID), fmt.Sprintf(`{"parent_id":%d}`, grandchild.ID), 409)
	other := a.department("Other", nil)
	data = a.request("PATCH", fmt.Sprintf("/departments/%d", child.ID), fmt.Sprintf(`{"parent_id":%d}`, other.ID), 200)
	var moved models.Department
	require.NoError(t, json.Unmarshal(data, &moved))
	require.NotNil(t, moved.ParentID)
	assert.Equal(t, other.ID, *moved.ParentID)
	assert.Equal(t, "Renamed", moved.Name)
}

func TestDepartmentAPIValidationAndConstraints(t *testing.T) {
	a := setupAPI(t)
	root := a.department("Root", nil)
	a.request("POST", "/departments/", `{"name":"Root"}`, 409)
	a.department("Child", &root.ID)
	a.request("POST", "/departments/", fmt.Sprintf(`{"name":"Child","parent_id":%d}`, root.ID), 409)
	other := a.department("Other", nil)
	a.department("Child", &other.ID) // same name under a different parent is valid
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/departments/", `{"name":"  "}`, 400},
		{"POST", "/departments/", `{`, 400},
		{"POST", "/departments/", `{"name":"Orphan","parent_id":999999}`, 404},
		{"GET", "/departments/999999", "", 404},
		{"GET", "/departments/invalid", "", 400},
		{"GET", "/departments/0", "", 400},
		{"POST", "/departments/999999/employees/", `{"full_name":"X","position":"Dev"}`, 404},
		{"POST", fmt.Sprintf("/departments/%d/employees/", root.ID), `{"full_name":"X","position":" "}`, 400},
		{"PATCH", fmt.Sprintf("/departments/%d", root.ID), fmt.Sprintf(`{"parent_id":%d}`, root.ID), 400},
		{"PATCH", "/departments/999999", `{"name":"Missing"}`, 404},
		{"DELETE", fmt.Sprintf("/departments/%d?mode=invalid", root.ID), "", 400},
		{"DELETE", fmt.Sprintf("/departments/%d?mode=reassign", root.ID), "", 400},
		{"DELETE", fmt.Sprintf("/departments/%d?mode=reassign&reassign_to_department_id=%d", root.ID, root.ID), "", 400},
		{"DELETE", "/departments/999999", "", 404},
	} {
		a.request(tc.method, tc.path, tc.body, tc.status)
	}
}

func TestDepartmentAPICascade(t *testing.T) {
	a := setupAPI(t)
	root := a.department("Root", nil)
	child := a.department("Child", &root.ID)
	grandchild := a.department("Grandchild", &child.ID)
	a.employee(root.ID)
	a.employee(grandchild.ID)
	a.request("DELETE", fmt.Sprintf("/departments/%d?mode=cascade", root.ID), "", 204)
	for _, table := range []string{"departments", "employees"} {
		var count int
		require.NoError(t, a.db.Get(&count, "SELECT count(*) FROM "+table))
		assert.Zero(t, count)
	}
}

func TestDepartmentAPIReassign(t *testing.T) {
	a := setupAPI(t)
	root := a.department("Root", nil)
	source := a.department("Source", &root.ID)
	child := a.department("Child", &source.ID)
	target := a.department("Target", nil)
	emp := a.employee(source.ID)
	a.request("DELETE", fmt.Sprintf("/departments/%d?mode=reassign&reassign_to_department_id=%d", source.ID, target.ID), "", 204)
	a.request("GET", fmt.Sprintf("/departments/%d", source.ID), "", 404)
	var parent, assigned uint
	require.NoError(t, a.db.Get(&parent, "SELECT parent_id FROM departments WHERE id = $1", child.ID))
	require.NoError(t, a.db.Get(&assigned, "SELECT department_id FROM employees WHERE id = $1", emp.ID))
	assert.Equal(t, root.ID, parent)
	assert.Equal(t, target.ID, assigned)
}

func TestDepartmentAPIReassignRollback(t *testing.T) {
	a := setupAPI(t)
	root := a.department("Root", nil)
	source := a.department("Source", &root.ID)
	a.department("Collision", &root.ID)
	child := a.department("Collision", &source.ID)
	target := a.department("Target", nil)
	emp := a.employee(source.ID)
	a.request("DELETE", fmt.Sprintf("/departments/%d?mode=reassign&reassign_to_department_id=%d", source.ID, target.ID), "", 409)
	var assigned, parent uint
	require.NoError(t, a.db.Get(&assigned, "SELECT department_id FROM employees WHERE id = $1", emp.ID))
	require.NoError(t, a.db.Get(&parent, "SELECT parent_id FROM departments WHERE id = $1", child.ID))
	assert.Equal(t, source.ID, assigned, "employee move must roll back when child promotion fails")
	assert.Equal(t, source.ID, parent)
	a.request("GET", fmt.Sprintf("/departments/%d", source.ID), "", 200)
}

func TestDepartmentAPIConcurrentMoves(t *testing.T) {
	a := setupAPI(t)
	first := a.department("First", nil)
	second := a.department("Second", nil)
	type result struct {
		status int
		err    error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, move := range [][2]uint{{first.ID, second.ID}, {second.ID, first.ID}} {
		wg.Add(1)
		go func(id, parent uint) {
			defer wg.Done()
			<-start
			req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/departments/%d", a.server.URL, id), bytes.NewBufferString(fmt.Sprintf(`{"parent_id":%d}`, parent)))
			if err != nil {
				results <- result{err: err}
				return
			}
			res, err := a.server.Client().Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			_, _ = io.Copy(io.Discard, res.Body)
			res.Body.Close()
			results <- result{status: res.StatusCode}
		}(move[0], move[1])
	}
	close(start)
	wg.Wait()
	close(results)
	statuses := make([]int, 0, 2)
	for res := range results {
		require.NoError(t, res.err)
		statuses = append(statuses, res.status)
	}
	assert.ElementsMatch(t, []int{200, 409}, statuses)
}
