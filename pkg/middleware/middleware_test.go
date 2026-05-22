package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/metrics/metricstest"
	"github.com/truefoundry/cruisekube/pkg/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthBasic_EnabledValidCreds_NoCreds401(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"}
	handler := AuthBasic(auth)

	r := gin.New()
	r.GET("/test", handler, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthBasic_EnabledValidCreds_WithCreds200(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"}
	handler := AuthBasic(auth)

	r := gin.New()
	r.GET("/test", handler, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "secret")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthBasic_Disabled_Passthrough200(t *testing.T) {
	auth := config.AuthConfig{Enabled: false, Username: "", Password: ""}
	handler := AuthBasic(auth)

	r := gin.New()
	r.GET("/test", handler, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthBasic_EnabledEmptyCreds_503(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "", Password: ""}
	handler := AuthBasic(auth)

	r := gin.New()
	r.GET("/test", handler, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestLoggerUsesUnmatchedRouteForMissingRouteTemplate(t *testing.T) {
	r := gin.New()
	r.Use(Logger(), RequestContext())

	beforeUnmatched := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "unmatched",
		"method": http.MethodGet,
		"status": "404",
	})
	beforeRawPath := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "/missing-route-for-metrics-test",
		"method": http.MethodGet,
		"status": "404",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing-route-for-metrics-test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unmatched route, got %d", w.Code)
	}

	if got := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "unmatched",
		"method": http.MethodGet,
		"status": "404",
	}); got-beforeUnmatched != 1 {
		t.Fatalf("expected unmatched route histogram count to increase by 1, before=%v after=%v", beforeUnmatched, got)
	}

	if got := metricstest.SampleValue(t, "cruisekube_http_server_request_duration_seconds_count", map[string]string{
		"route":  "/missing-route-for-metrics-test",
		"method": http.MethodGet,
		"status": "404",
	}); got != beforeRawPath {
		t.Fatalf("expected raw path histogram count to remain %v, got %v", beforeRawPath, got)
	}
}
