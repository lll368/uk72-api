package router

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

//go:embed web/default/dist/** web/classic/dist/**
var webRouterTestFS embed.FS

func newWebRouterTestAssets() ThemeAssets {
	return ThemeAssets{
		DefaultBuildFS:   webRouterTestFS,
		DefaultIndexPage: []byte("<!doctype html><title>default</title>"),
		ClassicBuildFS:   webRouterTestFS,
		ClassicIndexPage: []byte("<!doctype html><title>classic</title>"),
	}
}

func withStrictWebRateLimit(t *testing.T) {
	t.Helper()

	previousRedisEnabled := common.RedisEnabled
	previousWebRateLimitEnabled := common.GlobalWebRateLimitEnable
	previousWebRateLimitNum := common.GlobalWebRateLimitNum
	previousWebRateLimitDuration := common.GlobalWebRateLimitDuration
	previousTheme := common.GetTheme()

	common.RedisEnabled = false
	common.GlobalWebRateLimitEnable = true
	common.GlobalWebRateLimitNum = 1
	common.GlobalWebRateLimitDuration = 180
	common.SetTheme("default")

	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.GlobalWebRateLimitEnable = previousWebRateLimitEnabled
		common.GlobalWebRateLimitNum = previousWebRateLimitNum
		common.GlobalWebRateLimitDuration = previousWebRateLimitDuration
		common.SetTheme(previousTheme)
	})
}

func performWebRouterRequest(engine *gin.Engine, path string, ipSuffix int) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = fmt.Sprintf("203.0.113.%d:12345", ipSuffix)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestWebRateLimitDoesNotThrottleStaticAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withStrictWebRateLimit(t)
	engine := gin.New()
	SetWebRouter(engine, newWebRouterTestAssets())

	for i := 0; i < 3; i++ {
		recorder := performWebRouterRequest(engine, "/static/js/app.js", 41)

		require.Equal(t, http.StatusOK, recorder.Code)
	}
}

func TestStaticAssetsDoNotConsumeHtmlWebRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withStrictWebRateLimit(t)
	engine := gin.New()
	SetWebRouter(engine, newWebRouterTestAssets())

	staticRecorder := performWebRouterRequest(engine, "/static/js/app.js", 42)
	require.Equal(t, http.StatusOK, staticRecorder.Code)

	firstHtmlRecorder := performWebRouterRequest(engine, "/pricing", 42)
	require.Equal(t, http.StatusOK, firstHtmlRecorder.Code)

	secondHtmlRecorder := performWebRouterRequest(engine, "/pricing", 42)
	require.Equal(t, http.StatusTooManyRequests, secondHtmlRecorder.Code)
}

func TestIndexHtmlUsesWebRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withStrictWebRateLimit(t)
	engine := gin.New()
	SetWebRouter(engine, newWebRouterTestAssets())

	firstHtmlRecorder := performWebRouterRequest(engine, "/index.html", 43)
	require.NotEqual(t, http.StatusTooManyRequests, firstHtmlRecorder.Code)

	secondHtmlRecorder := performWebRouterRequest(engine, "/index.html", 43)
	require.Equal(t, http.StatusTooManyRequests, secondHtmlRecorder.Code)
}
