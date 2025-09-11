package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouterWithServer returns an http.Handler (Gin engine) with routes wired to the given Server.
func NewRouterWithServer(s *Server) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())

	// Health checks
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	// Webhook endpoint (support both paths for local and Vercel)
	r.POST("/webhook", gin.WrapH(s))
	r.POST("/api/webhook", gin.WrapH(s))

	return r
}

// RouterFromEnv creates a Server from env and returns a Gin router wired to it.
func RouterFromEnv() (http.Handler, error) {
	srv, err := ServerFromEnv()
	if err != nil {
		return nil, err
	}
	return NewRouterWithServer(srv), nil
}
