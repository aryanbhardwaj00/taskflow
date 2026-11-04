package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/blockaryan/taskflow/internal/handler"
)

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handler.NewHealthHandler()
	router := gin.New()
	router.GET("/health", h.HealthCheck)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "service available", body["status"])
	assert.Equal(t, "TaskFlow API", body["system"])
	assert.NotEmpty(t, body["timestamp"])
}
