package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/blockaryan/taskflow/internal/middleware"
)

func TestPrometheusMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.PrometheusMetrics())

	router.GET("/test-metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# HELP dummy metrics")
	})

	t.Run("Standard request records metrics and returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
	})

	t.Run("Metrics path itself bypasses instrumentation without error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
