package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateProject creates a new project with auto-generated API key
func (db *DB) CreateProject(name, slug, description string) (*Project, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	// Generate unique API key
	apiKey := generateUUID()

	now := time.Now().UnixMilli()

	result, err := db.conn.Exec(`
		INSERT INTO projects (name, slug, description, api_key, retention_days, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, name, slug, description, apiKey, 30, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get project id: %w", err)
	}

	return &Project{
		ID:            id,
		Name:          name,
		Slug:          slug,
		Description:   description,
		APIKey:        apiKey,
		RetentionDays: 30,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetProject retrieves a project by ID or slug
func (db *DB) GetProject(idOrSlug interface{}) (*Project, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	var project Project
	var err error

	switch v := idOrSlug.(type) {
	case int64:
		err = db.conn.QueryRow(`
			SELECT id, name, slug, description, api_key, retention_days, created_at, updated_at, deleted_at
			FROM projects WHERE id = ? AND deleted_at = 0
		`, v).Scan(&project.ID, &project.Name, &project.Slug, &project.Description,
			&project.APIKey, &project.RetentionDays, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt)
	case string:
		err = db.conn.QueryRow(`
			SELECT id, name, slug, description, api_key, retention_days, created_at, updated_at, deleted_at
			FROM projects WHERE slug = ? AND deleted_at = 0
		`, v).Scan(&project.ID, &project.Name, &project.Slug, &project.Description,
			&project.APIKey, &project.RetentionDays, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt)
	default:
		return nil, fmt.Errorf("invalid type for project lookup")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &project, nil
}

// ListProjects returns all projects the user has access to
func (db *DB) ListProjects(userID int64) ([]Project, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	// If no user ID provided (admin), return all projects
	// Otherwise, return projects where user is a member
	var rows *sql.Rows
	var err error

	if userID == 0 {
		rows, err = db.conn.Query(`
			SELECT id, name, slug, description, api_key, retention_days, created_at, updated_at, deleted_at
			FROM projects WHERE deleted_at = 0
			ORDER BY created_at DESC
		`)
	} else {
		rows, err = db.conn.Query(`
			SELECT DISTINCT p.id, p.name, p.slug, p.description, p.api_key, p.retention_days, p.created_at, p.updated_at, p.deleted_at
			FROM projects p
			INNER JOIN project_members pm ON p.id = pm.project_id
			WHERE pm.user_id = ? AND p.deleted_at = 0
			ORDER BY p.created_at DESC
		`, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description,
			&p.APIKey, &p.RetentionDays, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// UpdateProject updates project details
func (db *DB) UpdateProject(id int64, updates map[string]interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	setParts := []string{"updated_at = ?"}
	args := []interface{}{time.Now().UnixMilli()}

	if name, ok := updates["name"].(string); ok {
		setParts = append(setParts, "name = ?")
		args = append(args, name)
	}

	if description, ok := updates["description"].(string); ok {
		setParts = append(setParts, "description = ?")
		args = append(args, description)
	}

	if retentionDays, ok := updates["retention_days"].(int); ok {
		setParts = append(setParts, "retention_days = ?")
		args = append(args, retentionDays)
	}

	if len(setParts) == 1 {
		return fmt.Errorf("no fields to update")
	}

	args = append(args, id)

	query := "UPDATE projects SET " + stringJoin(setParts, ", ") + " WHERE id = ? AND deleted_at = 0"
	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

// DeleteProject soft deletes a project
func (db *DB) DeleteProject(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE projects SET deleted_at = ? WHERE id = ?
	`, time.Now().UnixMilli(), id)

	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}

// RegenerateApiKey generates a new API key for a project
func (db *DB) RegenerateApiKey(projectID int64) (string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return "", fmt.Errorf("database is closed")
	}

	newApiKey := generateUUID()

	_, err := db.conn.Exec(`
		UPDATE projects SET api_key = ?, updated_at = ? WHERE id = ? AND deleted_at = 0
	`, newApiKey, time.Now().UnixMilli(), projectID)

	if err != nil {
		return "", fmt.Errorf("failed to regenerate api key: %w", err)
	}

	return newApiKey, nil
}

// AddProjectMember adds a user to a project
func (db *DB) AddProjectMember(projectID, userID int64, role string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	now := time.Now().UnixMilli()

	_, err := db.conn.Exec(`
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, userID, role, now)

	if err != nil {
		return fmt.Errorf("failed to add project member: %w", err)
	}

	return nil
}

// RemoveProjectMember removes a user from a project
func (db *DB) RemoveProjectMember(projectID, userID int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		DELETE FROM project_members WHERE project_id = ? AND user_id = ?
	`, projectID, userID)

	if err != nil {
		return fmt.Errorf("failed to remove project member: %w", err)
	}

	return nil
}

// UpdateProjectMemberRole updates a member's role
func (db *DB) UpdateProjectMemberRole(projectID, userID int64, newRole string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE project_members SET role = ? WHERE project_id = ? AND user_id = ?
	`, newRole, projectID, userID)

	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	return nil
}

// GetProjectMembers returns all members of a project
func (db *DB) GetProjectMembers(projectID int64) ([]ProjectMember, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, project_id, user_id, role, created_at
		FROM project_members WHERE project_id = ?
		ORDER BY created_at ASC
	`, projectID)

	if err != nil {
		return nil, fmt.Errorf("failed to get project members: %w", err)
	}
	defer rows.Close()

	var members []ProjectMember
	for rows.Next() {
		var m ProjectMember
		err := rows.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project member: %w", err)
		}
		members = append(members, m)
	}

	return members, nil
}

// GetUserRole returns the user's role in a project
func (db *DB) GetUserRole(projectID, userID int64) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return "", fmt.Errorf("database is closed")
	}

	var role string
	err := db.conn.QueryRow(`
		SELECT role FROM project_members WHERE project_id = ? AND user_id = ?
	`, projectID, userID).Scan(&role)

	if err != nil {
		return "", fmt.Errorf("failed to get user role: %w", err)
	}

	return role, nil
}

// GetProjectByAPIKey retrieves a project by its API key
func (db *DB) GetProjectByAPIKey(apiKey string) (*Project, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	var project Project
	err := db.conn.QueryRow(`
		SELECT id, name, slug, description, api_key, retention_days, created_at, updated_at, deleted_at
		FROM projects WHERE api_key = ? AND deleted_at = 0
	`, apiKey).Scan(&project.ID, &project.Name, &project.Slug, &project.Description,
		&project.APIKey, &project.RetentionDays, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get project by api key: %w", err)
	}

	return &project, nil
}

// AutoCreateDefaultProject creates a default project if none exist
func (db *DB) AutoCreateDefaultProject() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	// Check if any projects exist
	var count int64
	err := db.conn.QueryRow("SELECT COUNT(*) FROM projects WHERE deleted_at = 0").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count projects: %w", err)
	}

	if count > 0 {
		return nil // Projects already exist
	}

	// Create default project
	apiKey := generateUUID()
	now := time.Now().UnixMilli()

	_, err = db.conn.Exec(`
		INSERT INTO projects (name, slug, description, api_key, retention_days, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Default Project", "default", "Default project for existing events", apiKey, 30, now, now)

	if err != nil {
		return fmt.Errorf("failed to create default project: %w", err)
	}

	// Get the created project ID
	var projectID int64
	err = db.conn.QueryRow("SELECT last_insert_rowid()").Scan(&projectID)
	if err != nil {
		return fmt.Errorf("failed to get created project id: %w", err)
	}

	// Associate all existing events/issues/alerts with the default project
	_, err = db.conn.Exec("UPDATE events SET project_id = ? WHERE project_id IS NULL", projectID)
	if err != nil {
		return fmt.Errorf("failed to migrate events to default project: %w", err)
	}

	_, err = db.conn.Exec("UPDATE issues SET project_id = ? WHERE project_id IS NULL", projectID)
	if err != nil {
		return fmt.Errorf("failed to migrate issues to default project: %w", err)
	}

	_, err = db.conn.Exec("UPDATE alert_rules SET project_id = ? WHERE project_id IS NULL", projectID)
	if err != nil {
		return fmt.Errorf("failed to migrate alert rules to default project: %w", err)
	}

	return nil
}