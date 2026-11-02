package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/gin-gonic/gin"
)

func RateLimiter(redisClient *infra.RedisClient, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		identifier := c.ClientIP()
		if userId, err := GetUserID(c); err == nil {
			identifier = userId.String()
		}

		windowSec := int64(window.Seconds())
		currentWindow := time.Now().Unix() / windowSec

		key := fmt.Sprintf("ratelimit:%s:%d", identifier, currentWindow)

		// Increment request count
		count, err := redisClient.Client.Incr(ctx, key).Result()
		if err != nil {
			// Fail open: If Redis fails, log warning and let request pass through
			log.Printf("RATE LIMIT WARNING: Redis error on Incr: %v\n", err)
			c.Next()
			return
		}
		// 4. Set key expiration on first request in the window
		if count == 1 {
			redisClient.Client.Expire(ctx, key, window+1*time.Second)
		}

		// 5. Calculate remaining quota
		remaining := limit - int(count)
		if remaining < 0 {
			remaining = 0
		}

		// 6. Set standard RFC rate limit headers
		ttl, _ := redisClient.Client.TTL(ctx, key).Result()
		resetSeconds := int(ttl.Seconds())
		if resetSeconds < 0 {
			resetSeconds = int(window.Seconds())
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.Itoa(resetSeconds))

		// 7. Reject if limit exceeded
		if count > int64(limit) {
			c.Header("Retry-After", strconv.Itoa(resetSeconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"message":     fmt.Sprintf("Rate limit of %d requests per %s exceeded", limit, window),
				"retry_after": resetSeconds,
			})
			return
		}

		c.Next()
	}
}
