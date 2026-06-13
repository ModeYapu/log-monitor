package handler

import (
	"net/http"
)

// OpenAPIHandler serves OpenAPI specification and Swagger UI
type OpenAPIHandler struct {
	spec []byte
}

// NewOpenAPIHandler creates a new OpenAPI handler
func NewOpenAPIHandler(spec []byte) *OpenAPIHandler {
	return &OpenAPIHandler{
		spec: spec,
	}
}

// GetSpec serves the OpenAPI JSON specification
func (h *OpenAPIHandler) GetSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(h.spec)
}

// GenerateOpenAPISpec generates the OpenAPI 3.0 specification for LogMonitor
func GenerateOpenAPISpec() []byte {
	spec := `{
  "openapi": "3.0.3",
  "info": {
    "title": "LogMonitor API",
    "description": "LogMonitor is a frontend error monitoring and performance analysis platform. This API provides endpoints for event ingestion, querying, admin operations, and E2E verification integrations.",
    "version": "1.0.0",
    "contact": {
      "name": "LogMonitor Team"
    }
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Development server"
    }
  ],
  "tags": [
    {"name": "ingestion", "description": "Event data ingestion endpoints"},
    {"name": "query", "description": "Log and analytics query endpoints"},
    {"name": "performance", "description": "Performance metrics and Web Vitals"},
    {"name": "issues", "description": "Issue tracking and management"},
    {"name": "alerts", "description": "Alert configuration and triggers"},
    {"name": "admin", "description": "Administrative operations"},
    {"name": "webhooks", "description": "Webhook management and E2E verifier integration"}
  ],
  "paths": {
    "/api/report": {
      "post": {
        "tags": ["ingestion"],
        "summary": "Report events from SDK",
        "description": "Accepts batch event reports from the LogMonitor SDK. Events are buffered and written to storage asynchronously.",
        "operationId": "reportEvents",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/ReportRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Events accepted",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ReportResponse"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"}
        }
      }
    },
    "/api/query/logs": {
      "get": {
        "tags": ["query"],
        "summary": "Query log events",
        "description": "Retrieve paginated log events with filtering options.",
        "operationId": "queryLogs",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "type", "in": "query", "schema": {"type": "string", "enum": ["error", "performance", "resource", "api_error", "user_action", "info", "warn"]}},
          {"name": "level", "in": "query", "schema": {"type": "string"}},
          {"name": "release", "in": "query", "schema": {"type": "string"}},
          {"name": "env", "in": "query", "schema": {"type": "string"}},
          {"name": "keyword", "in": "query", "schema": {"type": "string"}},
          {"name": "startTime", "in": "query", "schema": {"type": "integer", "format": "int64"}},
          {"name": "endTime", "in": "query", "schema": {"type": "integer", "format": "int64"}},
          {"name": "page", "in": "query", "schema": {"type": "integer", "default": 1}},
          {"name": "pageSize", "in": "query", "schema": {"type": "integer", "default": 50}}
        ],
        "responses": {
          "200": {
            "description": "Query results",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/LogsResponse"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/stats": {
      "get": {
        "tags": ["query"],
        "summary": "Get application statistics",
        "operationId": "getStats",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/StatsResponse"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/apps": {
      "get": {
        "tags": ["query"],
        "summary": "List all applications",
        "operationId": "listApps",
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {"$ref": "#/components/schemas/AppInfo"}
                }
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/top": {
      "get": {
        "tags": ["query"],
        "summary": "Get top N items by various metrics",
        "description": "Retrieve top errors, pages, releases, or browsers by count, users, impact, or recency.",
        "operationId": "queryTop",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "type", "in": "query", "schema": {"type": "string", "enum": ["errors", "pages", "releases", "browsers"]}},
          {"name": "orderBy", "in": "query", "schema": {"type": "string", "enum": ["count", "users", "impact", "recent", "regression"]}},
          {"name": "limit", "in": "query", "schema": {"type": "integer", "default": 20}},
          {"name": "env", "in": "query", "schema": {"type": "string"}},
          {"name": "release", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/TopNResponse"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/export": {
      "get": {
        "tags": ["query"],
        "summary": "Export events",
        "description": "Export events in JSON or CSV format.",
        "operationId": "exportEvents",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "format", "in": "query", "schema": {"type": "string", "enum": ["json", "csv"]}}
        ],
        "responses": {
          "200": {
            "description": "Export file",
            "content": {
              "application/json": {
                "schema": {"type": "file"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/performance/summary": {
      "get": {
        "tags": ["performance"],
        "summary": "Get performance summary",
        "operationId": "getPerformanceSummary",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/PerformanceSummary"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/performance/trend": {
      "get": {
        "tags": ["performance"],
        "summary": "Get performance trend",
        "operationId": "getPerformanceTrend",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/PerformanceTrend"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/performance/regression": {
      "get": {
        "tags": ["performance"],
        "summary": "Detect performance regression",
        "operationId": "getPerformanceRegression",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "baselineRelease", "in": "query", "schema": {"type": "string"}},
          {"name": "currentRelease", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/RegressionReport"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/issues": {
      "get": {
        "tags": ["issues"],
        "summary": "List issues",
        "operationId": "listIssues",
        "parameters": [
          {"name": "appId", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "status", "in": "query", "schema": {"type": "string", "enum": ["open", "resolved", "ignored"]}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"type": "array", "items": {"$ref": "#/components/schemas/Issue"}}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/issues/{id}": {
      "get": {
        "tags": ["issues"],
        "summary": "Get issue details",
        "operationId": "getIssue",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/Issue"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/issues/{id}?action=resolve": {
      "post": {
        "tags": ["issues"],
        "summary": "Resolve an issue",
        "operationId": "resolveIssue",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/Issue"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/query/alerts": {
      "get": {
        "tags": ["alerts"],
        "summary": "List alert rules",
        "operationId": "listAlerts",
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"type": "array", "items": {"$ref": "#/components/schemas/AlertRule"}}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      },
      "post": {
        "tags": ["alerts"],
        "summary": "Create alert rule",
        "operationId": "createAlert",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AlertRuleCreate"}
            }
          }
        },
        "responses": {
          "201": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AlertRule"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    },
    "/api/admin/storage/stats": {
      "get": {
        "tags": ["admin"],
        "summary": "Get storage statistics",
        "operationId": "getStorageStats",
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/StorageStats"}
              }
            }
          }
        },
        "security": [{"bearerAuth": [], "adminRole": []}]
      }
    },
    "/api/admin/audit-logs": {
      "get": {
        "tags": ["admin"],
        "summary": "Get audit logs",
        "operationId": "getAuditLogs",
        "parameters": [
          {"name": "page", "in": "query", "schema": {"type": "integer", "default": 1}},
          {"name": "pageSize", "in": "query", "schema": {"type": "integer", "default": 50}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AuditLogsResponse"}
              }
            }
          }
        },
        "security": [{"bearerAuth": [], "adminRole": []}]
      }
    },
    "/api/admin/webhooks": {
      "get": {
        "tags": ["webhooks"],
        "summary": "List webhooks",
        "operationId": "listWebhooks",
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"type": "array", "items": {"$ref": "#/components/schemas/Webhook"}}
              }
            }
          }
        },
        "security": [{"bearerAuth": [], "adminRole": []}]
      },
      "post": {
        "tags": ["webhooks"],
        "summary": "Create webhook",
        "operationId": "createWebhook",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/WebhookCreate"}
            }
          }
        },
        "responses": {
          "201": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/Webhook"}
              }
            }
          }
        },
        "security": [{"bearerAuth": [], "adminRole": []}]
      }
    },
    "/api/webhooks/e2e-verifier": {
      "post": {
        "tags": ["webhooks"],
        "summary": "Receive E2E verification results",
        "description": "Webhook endpoint for E2E Verifier to report test results. Requires API key authentication via X-API-Key header.",
        "operationId": "e2eVerificationResult",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/VerificationResult"}
            }
          }
        },
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {
                  "success": {"type": "boolean"},
                  "message": {"type": "string"}
                }}
              }
            }
          }
        },
        "security": [{"apiKeyAuth": []}]
      }
    },
    "/api/webhooks/e2e-verifier/results": {
      "get": {
        "tags": ["webhooks"],
        "summary": "Get E2E verification results",
        "operationId": "getVerificationResults",
        "parameters": [
          {"name": "site", "in": "query", "schema": {"type": "string"}},
          {"name": "limit", "in": "query", "schema": {"type": "integer", "default": 10}}
        ],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/VerificationResultsResponse"}
              }
            }
          }
        },
        "security": [{"bearerAuth": []}]
      }
    }
  },
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      },
      "apiKeyAuth": {
        "type": "apiKey",
        "in": "header",
        "name": "X-API-Key"
      },
      "adminRole": {
        "type": "apiKey",
        "in": "header",
        "name": "X-Role",
        "description": "Requires admin role"
      }
    },
    "schemas": {
      "ReportRequest": {
        "type": "object",
        "required": ["appId", "events"],
        "properties": {
          "appId": {"type": "string"},
          "release": {"type": "string"},
          "projectId": {"type": "integer"},
          "apiKey": {"type": "string"},
          "events": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/Event"}
          }
        }
      },
      "Event": {
        "type": "object",
        "properties": {
          "appId": {"type": "string"},
          "release": {"type": "string"},
          "env": {"type": "string"},
          "buildId": {"type": "string"},
          "userId": {"type": "string"},
          "sessionId": {"type": "string"},
          "type": {"type": "string", "enum": ["error", "performance", "resource", "api_error", "user_action", "info", "warn"]},
          "level": {"type": "string"},
          "message": {"type": "string"},
          "stack": {"type": "string"},
          "url": {"type": "string"},
          "line": {"type": "integer"},
          "col": {"type": "integer"},
          "tags": {"type": "object"},
          "extra": {"type": "object"},
          "ua": {"type": "string"},
          "screen": {"type": "string"},
          "viewport": {"type": "string"},
          "performance": {"type": "object"},
          "ip": {"type": "string"},
          "timestamp": {"type": "integer", "format": "int64"}
        }
      },
      "ReportResponse": {
        "type": "object",
        "properties": {
          "success": {"type": "boolean"},
          "count": {"type": "integer"}
        }
      },
      "LogsResponse": {
        "type": "object",
        "properties": {
          "total": {"type": "integer"},
          "page": {"type": "integer"},
          "size": {"type": "integer"},
          "data": {"type": "array", "items": {"$ref": "#/components/schemas/Event"}}
        }
      },
      "StatsResponse": {
        "type": "object",
        "properties": {
          "totalEvents": {"type": "integer"},
          "errorCount": {"type": "integer"},
          "warnCount": {"type": "integer"},
          "infoCount": {"type": "integer"},
          "topErrors": {"type": "array", "items": {"$ref": "#/components/schemas/ErrorStat"}},
          "errorTrend": {"type": "array", "items": {"$ref": "#/components/schemas/TrendPoint"}}
        }
      },
      "ErrorStat": {
        "type": "object",
        "properties": {
          "message": {"type": "string"},
          "count": {"type": "integer"},
          "lastSeen": {"type": "integer", "format": "int64"}
        }
      },
      "TrendPoint": {
        "type": "object",
        "properties": {
          "timestamp": {"type": "integer", "format": "int64"},
          "count": {"type": "integer"}
        }
      },
      "AppInfo": {
        "type": "object",
        "properties": {
          "appId": {"type": "string"},
          "release": {"type": "string"},
          "firstSeen": {"type": "integer", "format": "int64"},
          "lastSeen": {"type": "integer", "format": "int64"},
          "errorCount": {"type": "integer"},
          "totalEvents": {"type": "integer"}
        }
      },
      "TopNResponse": {
        "type": "object",
        "properties": {
          "type": {"type": "string"},
          "data": {"type": "array", "items": {"$ref": "#/components/schemas/TopNItem"}}
        }
      },
      "TopNItem": {
        "type": "object",
        "properties": {
          "key": {"type": "string"},
          "count": {"type": "integer"},
          "users": {"type": "integer"},
          "lastSeen": {"type": "integer", "format": "int64"},
          "firstSeen": {"type": "integer", "format": "int64"},
          "isNew": {"type": "boolean"},
          "impactScore": {"type": "integer"}
        }
      },
      "PerformanceSummary": {
        "type": "object",
        "properties": {
          "metrics": {"type": "object"},
          "percentiles": {"type": "object"}
        }
      },
      "PerformanceTrend": {
        "type": "object",
        "properties": {
          "points": {"type": "array", "items": {"type": "object"}}
        }
      },
      "RegressionReport": {
        "type": "object",
        "properties": {
          "hasRegression": {"type": "boolean"},
          "affectedMetrics": {"type": "array", "items": {"type": "string"}},
          "details": {"type": "array", "items": {"type": "object"}}
        }
      },
      "Issue": {
        "type": "object",
        "properties": {
          "id": {"type": "integer"},
          "fingerprint": {"type": "string"},
          "appId": {"type": "string"},
          "title": {"type": "string"},
          "status": {"type": "string", "enum": ["open", "resolved", "ignored"]},
          "priority": {"type": "string", "enum": ["low", "medium", "high", "critical"]},
          "firstSeenAt": {"type": "integer", "format": "int64"},
          "lastSeenAt": {"type": "integer", "format": "int64"},
          "eventCount": {"type": "integer"},
          "userCount": {"type": "integer"}
        }
      },
      "AlertRule": {
        "type": "object",
        "properties": {
          "id": {"type": "integer"},
          "name": {"type": "string"},
          "conditionType": {"type": "string"},
          "conditionConfig": {"type": "object"},
          "notifyType": {"type": "string"},
          "notifyConfig": {"type": "object"},
          "enabled": {"type": "boolean"}
        }
      },
      "AlertRuleCreate": {
        "type": "object",
        "required": ["name", "conditionType", "notifyType"],
        "properties": {
          "name": {"type": "string"},
          "conditionType": {"type": "string"},
          "conditionConfig": {"type": "object"},
          "notifyType": {"type": "string"},
          "notifyConfig": {"type": "object"},
          "enabled": {"type": "boolean"}
        }
      },
      "StorageStats": {
        "type": "object",
        "properties": {
          "dbSize": {"type": "integer"},
          "totalEvents": {"type": "integer"},
          "retentionDays": {"type": "integer"}
        }
      },
      "AuditLogsResponse": {
        "type": "object",
        "properties": {
          "logs": {"type": "array", "items": {"$ref": "#/components/schemas/AuditLog"}},
          "total": {"type": "integer"}
        }
      },
      "AuditLog": {
        "type": "object",
        "properties": {
          "id": {"type": "integer"},
          "userId": {"type": "integer"},
          "username": {"type": "string"},
          "action": {"type": "string"},
          "resource": {"type": "string"},
          "resourceId": {"type": "integer"},
          "detail": {"type": "string"},
          "ip": {"type": "string"},
          "createdAt": {"type": "integer", "format": "int64"}
        }
      },
      "Webhook": {
        "type": "object",
        "properties": {
          "id": {"type": "integer"},
          "projectId": {"type": "integer"},
          "name": {"type": "string"},
          "url": {"type": "string"},
          "events": {"type": "array", "items": {"type": "string"}},
          "enabled": {"type": "boolean"}
        }
      },
      "WebhookCreate": {
        "type": "object",
        "required": ["name", "url", "events"],
        "properties": {
          "name": {"type": "string"},
          "url": {"type": "string"},
          "events": {"type": "array", "items": {"type": "string"}},
          "enabled": {"type": "boolean"}
        }
      },
      "VerificationResult": {
        "type": "object",
        "required": ["site", "release", "status"],
        "properties": {
          "site": {"type": "string", "description": "Site identifier (e.g., 'travel-planner')"},
          "release": {"type": "string", "description": "Release version (e.g., 'v1.2.3')"},
          "status": {"type": "string", "enum": ["pass", "fail"]},
          "score": {"type": "number", "minimum": 0, "maximum": 10, "description": "Overall score (0-10)"},
          "checks": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/CheckResult"}
          },
          "timestamp": {"type": "integer", "format": "int64"}
        }
      },
      "CheckResult": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "status": {"type": "string", "enum": ["pass", "fail"]},
          "message": {"type": "string"},
          "duration": {"type": "integer", "description": "Duration in milliseconds"}
        }
      },
      "VerificationResultsResponse": {
        "type": "object",
        "properties": {
          "site": {"type": "string"},
          "count": {"type": "integer"},
          "results": {"type": "array", "items": {"$ref": "#/components/schemas/VerificationResult"}}
        }
      }
    },
    "responses": {
      "BadRequest": {
        "description": "Bad request",
        "content": {
          "application/json": {
            "schema": {"type": "object", "properties": {
              "error": {"type": "string"}
            }}
          }
        }
      }
    }
  }
}`
	return []byte(spec)
}

// GetSwaggerUI serves the Swagger UI HTML page
func (h *OpenAPIHandler) GetSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(swaggerUIHTML))
}

// swaggerUIHTML is the inline Swagger UI HTML
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LogMonitor API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui.css">
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            padding: 0;
            font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
        }
        .topbar {
            background-color: #1f2937;
            padding: 15px 0;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .topbar-wrapper {
            max-width: 1460px;
            margin: 0 auto;
            padding: 0 20px;
        }
        .topbar-wrapper::after {
            content: "";
            clear: both;
            display: table;
        }
        .topbar-wrapper .link {
            display: inline-block;
            float: left;
            margin: 0;
        }
        .topbar-wrapper .link img {
            height: 30px;
            vertical-align: middle;
        }
        .topbar-wrapper .link span {
            color: #fff;
            font-size: 20px;
            font-weight: 600;
            margin-left: 10px;
            vertical-align: middle;
        }
        #swagger-ui {
            max-width: 1460px;
            margin: 0 auto;
            padding: 20px;
        }
        .swagger-ui .topbar {
            display: none;
        }
        .swagger-ui .info {
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="topbar">
        <div class="topbar-wrapper">
            <a class="link" href="/">
                <span>LogMonitor API Documentation</span>
            </a>
        </div>
    </div>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "/api/docs",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                docExpansion: "list",
                filter: true,
                showRequestDuration: true,
                tryItOutEnabled: true
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`
