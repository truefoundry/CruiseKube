package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleLogin_Disabled_404(t *testing.T) {
	auth := config.AuthConfig{Enabled: false, Username: "admin", Password: "secret"}
	handler := HandleLogin(auth)

	r := gin.New()
	r.POST("/login", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleLogin_EnabledEmptyCreds_503(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "", Password: ""}
	handler := HandleLogin(auth)

	r := gin.New()
	r.POST("/login", handler)

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleLogin_ValidCreds_200(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"}
	handler := HandleLogin(auth)

	r := gin.New()
	r.POST("/login", handler)

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp loginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.TokenType != "Basic" {
		t.Errorf("expected token_type Basic, got %s", resp.TokenType)
	}
}

func TestHandleLogin_WrongPassword_401(t *testing.T) {
	auth := config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"}
	handler := HandleLogin(auth)

	r := gin.New()
	r.POST("/login", handler)

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
