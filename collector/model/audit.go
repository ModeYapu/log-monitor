package model

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         int64  `json:"id"`
	ProjectID  int64  `json:"project_id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	Action     string `json:"action"`      // create/update/delete/login/export/view
	Resource   string `json:"resource"`    // project/user/alert/issue/sourcemap/event
	ResourceID string `json:"resource_id"` // ID of the affected resource
	Detail     string `json:"detail"`      // Additional details about the action
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  int64  `json:"created_at"`
}

// AuditFilter represents filter parameters for querying audit logs
type AuditFilter struct {
	ProjectID int64
	UserID    int64
	Action    string
	Resource  string
	StartDate int64
	EndDate   int64
	Page      int
	PageSize  int
}

// AuditAction constants
const (
	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
	AuditActionLogin  = "login"
	AuditActionLogout = "logout"
	AuditActionExport = "export"
	AuditActionView   = "view"
)

// AuditResource constants
const (
	AuditResourceProject   = "project"
	AuditResourceUser      = "user"
	AuditResourceAlert     = "alert"
	AuditResourceIssue     = "issue"
	AuditResourceSourceMap = "sourcemap"
	AuditResourceEvent     = "event"
	AuditResourceWebhook   = "webhook"
)
