package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// MaxMemoryLimit: 32MB maximum ephemeral memory buffer
const MaxMemoryLimit = 32 << 20

// SetupCORS configures CORS allowing local Swagger UI, dev frontends, and external clients.
func SetupCORS() gin.HandlerFunc {
	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return true // Allow all origins for Swagger UI, localhost:8080, 127.0.0.1, and dev frontends
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Session-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Session-ID", "X-Response-Time"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	return cors.New(config)
}

// MemoryLimiterMiddleware rejects requests exceeding 32MB to enforce Ephemeral RAM bounds.
func MemoryLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxMemoryLimit)
		c.Next()
	}
}

// RequestLogger logs incoming API requests and processing latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[AZAMAN-ENGINE] %s %s | %d | %v | %s",
			c.Request.Method,
			path,
			status,
			latency,
			c.ClientIP(),
		)
	}
}
