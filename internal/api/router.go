package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin engine with all middleware and API routes.
func SetupRouter() *gin.Engine {
	r := gin.New()

	// Ephemeral RAM: MemoryLimiterMiddleware caps the whole body at 32MB. The multipart in-memory
	// threshold must sit above that cap, otherwise large file parts spill to OS temp files on disk.
	r.MaxMultipartMemory = MaxMemoryLimit + (1 << 20)

	// Standard middleware stack
	r.Use(gin.Recovery())
	r.Use(RequestLogger())
	r.Use(MemoryLimiterMiddleware())
	r.Use(SetupCORS())

	// Interactive Web UI Playground & Swagger Documentation
	r.GET("/", HandleDashboard)
	r.GET("/swagger", HandleSwaggerUI)
	r.GET("/docs", HandleSwaggerUI)
	r.GET("/api/openapi.json", HandleOpenAPISpec)

	// Health and diagnostics
	r.GET("/health", HandleHealth)

	// API v1 Routes
	v1 := r.Group("/api/v1")
	{
		shazamGroup := v1.Group("/shazam")
		{
			shazamGroup.POST("/audit", HandleAudit)
			shazamGroup.POST("/decode-memo", HandleDecodeMemo)
		}

		v1.GET("/merchants", HandleGetMerchants)
	}

	return r
}
