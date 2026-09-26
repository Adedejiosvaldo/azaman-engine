package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSwaggerUI serves the interactive Swagger documentation UI.
func HandleSwaggerUI(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Azaman Engine API - Swagger Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" />
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/api/openapi.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// HandleOpenAPISpec serves the OpenAPI 3.0 specification JSON.
func HandleOpenAPISpec(c *gin.Context) {
	spec := gin.H{
		"openapi": "3.0.3",
		"info": gin.H{
			"title":       "Azaman Shazam Extraction Engine",
			"description": "Ephemeral, sub-second financial intelligence engine built to decode cryptic Nigerian banking memos, spotlight silent cash leaks, and compute runway.",
			"version":     "1.0.0",
		},
		"servers": []gin.H{
			{"url": "http://localhost:8080", "description": "Local Engine Server"},
		},
		"paths": gin.H{
			"/api/v1/shazam/audit": gin.H{
				"post": gin.H{
					"summary":     "Audit Bank Statement (PDF, CSV, XLSX)",
					"description": "Upload a PDF or CSV bank statement. Evaluates transactions in RAM bounded by 32MB without disk storage.",
					"requestBody": gin.H{
						"required": true,
						"content": gin.H{
							"multipart/form-data": gin.H{
								"schema": gin.H{
									"type": "object",
									"required": []string{"statement"},
									"properties": gin.H{
										"statement": gin.H{
											"type":        "string",
											"format":      "binary",
											"description": "Bank statement file (PDF, CSV, TSV, XLSX)",
										},
										"password": gin.H{
											"type":        "string",
											"description": "Optional password for encrypted PDFs (e.g. account or phone number)",
										},
										"bank": gin.H{
											"type":        "string",
											"description": "Optional bank override (GTBank, Kuda Bank, OPay, Moniepoint, Access Bank, Geegpay USD)",
										},
									},
								},
							},
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Comprehensive Audit Report with KPIs, silent leaks, and enriched transactions",
						},
						"400": gin.H{"description": "Missing statement file"},
						"422": gin.H{"description": "Extraction or parsing error"},
					},
				},
			},
			"/api/v1/shazam/decode-memo": gin.H{
				"post": gin.H{
					"summary":     "Decode Single Memo (Sub-5ms)",
					"description": "Cleans clearinghouse tags, matches merchant, and returns category with confidence scoring.",
					"requestBody": gin.H{
						"required": true,
						"content": gin.H{
							"application/json": gin.H{
								"schema": gin.H{
									"type": "object",
									"required": []string{"memo"},
									"properties": gin.H{
										"memo":     gin.H{"type": "string", "example": "TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00"},
										"amount":   gin.H{"type": "number", "example": 8500},
										"date":     gin.H{"type": "string", "example": "2024-01-15 23:15:00"},
										"currency": gin.H{"type": "string", "example": "NGN"},
									},
								},
							},
						},
					},
					"responses": gin.H{
						"200": gin.H{"description": "Decoded transaction details"},
					},
				},
			},
			"/api/v1/merchants": gin.H{
				"get": gin.H{
					"summary":     "List Merchant Registry Taxonomy",
					"description": "Lists recognized Nigerian and global merchant rules and classifications.",
					"responses": gin.H{
						"200": gin.H{"description": "List of merchants and categories"},
					},
				},
			},
			"/health": gin.H{
				"get": gin.H{
					"summary":     "Health & Ephemeral Memory Diagnostics",
					"description": "Returns engine status, uptime, Laya AI sidecar connection, and RAM memory stats.",
					"responses": gin.H{
						"200": gin.H{"description": "System health and memory statistics"},
					},
				},
			},
		},
	}
	c.JSON(http.StatusOK, spec)
}
