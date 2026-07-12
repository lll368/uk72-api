package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetApiRouterDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("SetApiRouter panicked: %v", recovered)
		}
	}()

	SetApiRouter(engine)
}

func TestPiggyWithdrawScanRouteUsesConflictFreePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	if !hasRoute(engine, "POST", "/api/wallet/admin/piggy/withdraws/scan") {
		t.Fatal("expected Piggy withdraw scan route to use /api/wallet/admin/piggy/withdraws/scan")
	}
	if hasRoute(engine, "POST", "/api/wallet/admin/withdraws/piggy/scan") {
		t.Fatal("old Piggy withdraw scan route conflicts with /withdraws/:id routes")
	}
}

func TestPiggyContractPreviewRouteUsesWalletAuthPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	assertRouteHandler(t, engine, "POST", "/api/wallet/withdraw/piggy/contract-preview", "GetWalletPiggyContractPreview")
}

func TestEpayRoutesUseProductionHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	assertRouteHandler(t, engine, "POST", "/api/user/pay", "RequestEpay")
	assertRouteHandler(t, engine, "POST", "/api/vip/epay/pay", "VipActivationRequestEpay")
}

func hasRoute(engine *gin.Engine, method string, path string) bool {
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func assertRouteHandler(t *testing.T, engine *gin.Engine, method string, path string, want string) {
	t.Helper()
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			if !strings.HasSuffix(route.Handler, "."+want) {
				t.Fatalf("route %s %s handler = %s, want %s", method, path, route.Handler, want)
			}
			return
		}
	}
	t.Fatalf("route %s %s not found", method, path)
}
