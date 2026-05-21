-- +goose Up
-- Уникальность имени в пределах одного родителя
CREATE UNIQUE INDEX idx_dept_name_parent_id ON departments (name, parent_id) WHERE parent_id IS NOT NULL;
CREATE UNIQUE INDEX idx_dept_name_parent_null ON departments (name) WHERE parent_id IS NULL;

-- +goose Down
DROP INDEX idx_dept_name_parent_id;
DROP INDEX idx_dept_name_parent_null;
