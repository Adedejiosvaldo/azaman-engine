package api

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// MaxMemoryLimit: 32MB maximum ephemeral memory buffer
const MaxMemoryLimit = 32 << 20

// SetupCORS configures CORS allowing local Swagger UI, dev frontends, and external clients.
// Set ALLOWED_ORIGINS (comma-separated, e.g. "https://azaman.example.com") in production;
// when unset, all origins are allowed for local development.
func SetupCORS() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[strings.TrimSuffix(o, "/")] = true
		}
	}
	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return len(allowed) == 0 || allowed[origin]
		},
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Session-ID"},
		ExposeHeaders: []string{"Content-Length", "X-Session-ID", "X-Response-Time"},
		// Credentials only for an explicit allowlist: reflecting any origin with credentials
		// would let every website make authenticated requests.
		AllowCredentials: len(allowed) > 0,
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
