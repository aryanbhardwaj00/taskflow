package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves API health status checks.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "service available",
		"timestamp": time.Now().UTC().Format(time.RFC1123),
		"system":    "TaskFlow API",
	})
}
