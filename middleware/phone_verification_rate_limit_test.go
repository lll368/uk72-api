package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhoneVerificationRateLimitUsesI18n(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	router.Use(PhoneVerificationRateLimit())
	router.POST("/phone/verification", func(c *gin.Context) {
		common.ApiSuccess(c, nil)
	})

	var lastRecorder *httptest.ResponseRecorder
	for i := 0; i < PhoneVerificationMaxRequests+1; i++ {
		req := httptest.NewRequest(http.MethodPost, "/phone/verification", bytes.NewBufferString(`{}`))
		req.Header.Set("Accept-Language", "en")
		req.RemoteAddr = "203.0.113.10:12345"
		lastRecorder = httptest.NewRecorder()
		router.ServeHTTP(lastRecorder, req)
	}

	require.NotNil(t, lastRecorder)
	assert.Equal(t, http.StatusTooManyRequests, lastRecorder.Code)
	assert.Contains(t, strings.ToLower(lastRecorder.Body.String()), "too frequently")
}
