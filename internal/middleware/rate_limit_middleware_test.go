package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/middleware"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *infra.RedisClient) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, &infra.RedisClient{Client: rdb}
}

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, redisClient := setupTestRedis(t)
	defer mr.Close()

	router := gin.New()
	limit := 3
	window := 10 * time.Second

	router.Use(middleware.RateLimiter(redisClient, limit, window))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "allowed"})
	})

	for i := 1; i <= limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "3", w.Header().Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
	}
}

func TestRateLimiter_BlocksRequestsExceedingLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, redisClient := setupTestRedis(t)
	defer mr.Close()

	router := gin.New()
	limit := 2
	window := 10 * time.Second

	router.Use(middleware.RateLimiter(redisClient, limit, window))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "allowed"})
	})

	// First 2 requests succeed
	for i := 1; i <= limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 3rd request should be blocked with 429
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("Retry-After"))

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "Too many requests", body["error"])
}

func TestRateLimiter_AuthenticatedUserRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, redisClient := setupTestRedis(t)
	defer mr.Close()

	router := gin.New()
	limit := 1
	window := 10 * time.Second

	userID := uuid.New()

	// Dummy auth injector middleware
	router.Use(func(c *gin.Context) {
		c.Set(middleware.ContextUserIDKey, userID)
		c.Next()
	})
	router.Use(middleware.RateLimiter(redisClient, limit, window))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "allowed"})
	})

	// 1st request succeeds
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// 2nd request for same user is blocked
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestRateLimiter_FailOpenWhenRedisUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Close miniredis immediately to simulate Redis outage
	mr, redisClient := setupTestRedis(t)
	mr.Close()

	router := gin.New()
	router.Use(middleware.RateLimiter(redisClient, 5, 10*time.Second))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "allowed"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail open: let request pass with 200 OK
	assert.Equal(t, http.StatusOK, w.Code)
}
