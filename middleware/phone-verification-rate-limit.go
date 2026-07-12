package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
)

const (
	PhoneVerificationRateLimitMark = "PV"
	PhoneVerificationMaxRequests   = 2
	PhoneVerificationDuration      = 30
)

func redisPhoneVerificationRateLimiter(c *gin.Context) {
	ctx := context.Background()
	key := "phoneVerification:" + PhoneVerificationRateLimitMark + ":" + c.ClientIP()

	count, err := common.RDB.Incr(ctx, key).Result()
	if err != nil {
		memoryPhoneVerificationRateLimiter(c)
		return
	}
	if count == 1 {
		_ = common.RDB.Expire(ctx, key, time.Duration(PhoneVerificationDuration)*time.Second).Err()
	}
	if count <= int64(PhoneVerificationMaxRequests) {
		c.Next()
		return
	}

	c.JSON(http.StatusTooManyRequests, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, i18n.MsgUserPhoneVerificationFrequent),
	})
	c.Abort()
}

func memoryPhoneVerificationRateLimiter(c *gin.Context) {
	key := PhoneVerificationRateLimitMark + ":" + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, PhoneVerificationMaxRequests, PhoneVerificationDuration) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgUserPhoneVerificationFrequent),
		})
		c.Abort()
		return
	}
	c.Next()
}

// PhoneVerificationRateLimit 对手机号验证码发送接口做 IP 级短窗口限流。
func PhoneVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.RedisEnabled {
			redisPhoneVerificationRateLimiter(c)
			return
		}
		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		memoryPhoneVerificationRateLimiter(c)
	}
}
