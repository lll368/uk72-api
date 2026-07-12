package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPiggyTaxTrialRateLimitDoesNotShareCriticalBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	originalCriticalEnabled := common.CriticalRateLimitEnable
	originalCriticalNum := common.CriticalRateLimitNum
	originalCriticalDuration := common.CriticalRateLimitDuration
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.CriticalRateLimitEnable = originalCriticalEnabled
		common.CriticalRateLimitNum = originalCriticalNum
		common.CriticalRateLimitDuration = originalCriticalDuration
		inMemoryRateLimiter = common.InMemoryRateLimiter{}
	})
	common.RedisEnabled = false
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 3600
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	router.GET("/critical", CriticalRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/trial", func(c *gin.Context) {
		c.Set("id", 9001)
	}, PiggyTaxTrialRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	firstCritical := httptest.NewRecorder()
	router.ServeHTTP(firstCritical, httptest.NewRequest(http.MethodGet, "/critical", nil))
	assert.Equal(t, http.StatusOK, firstCritical.Code)

	secondCritical := httptest.NewRecorder()
	router.ServeHTTP(secondCritical, httptest.NewRequest(http.MethodGet, "/critical", nil))
	assert.Equal(t, http.StatusTooManyRequests, secondCritical.Code)

	trial := httptest.NewRecorder()
	router.ServeHTTP(trial, httptest.NewRequest(http.MethodGet, "/trial", nil))
	assert.Equal(t, http.StatusOK, trial.Code)
}
