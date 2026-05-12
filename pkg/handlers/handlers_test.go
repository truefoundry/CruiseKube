package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleRoot_AuthEnabledTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := HandleRoot(true)

	r := gin.New()
	r.GET("/", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	authEnabled, ok := body["auth_enabled"]
	if !ok {
		t.Fatal("auth_enabled key not found in response")
	}
	if authEnabled != true {
		t.Errorf("expected auth_enabled=true, got %v", authEnabled)
	}
}

func TestHandleRoot_AuthEnabledFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := HandleRoot(false)

	r := gin.New()
	r.GET("/", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	authEnabled, ok := body["auth_enabled"]
	if !ok {
		t.Fatal("auth_enabled key not found in response")
	}
	if authEnabled != false {
		t.Errorf("expected auth_enabled=false, got %v", authEnabled)
	}
}
