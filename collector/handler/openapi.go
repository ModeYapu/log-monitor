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

// GetSpec serves the OpenAPI YAML specification
func (h *OpenAPIHandler) GetSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(h.spec)
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